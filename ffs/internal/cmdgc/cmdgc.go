package cmdgc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/creachadair/command"
	"github.com/creachadair/ffs/blob"
	"github.com/creachadair/ffs/file/root"
	"github.com/creachadair/ffs/index"
	"github.com/creachadair/misctools/ffs/config"
	"github.com/creachadair/taskgroup"
)

var Command = &command.C{
	Name:  "gc",
	Usage: "<root-key> <root-key>...",
	Help:  "Garbage-collect blobs not reachable from known roots",

	Run: func(env *command.Env, args []string) error {
		keys, err := config.RootKeys(args)
		if err != nil {
			return err
		} else if len(keys) == 0 {
			return errors.New("at least one root key is required")
		}

		cfg := env.Config.(*config.Settings)
		ctx, cancel := context.WithCancel(cfg.Context)
		return cfg.WithStore(cfg.Context, func(s blob.CAS) error {
			n, err := s.Len(ctx)
			if err != nil {
				return err
			} else if n == 0 {
				return errors.New("the store is empty")
			}
			idx := index.New(int(n), &index.Options{FalsePositiveRate: 0.005})

			// Mark phase: Scan all roots.
			for _, key := range keys {
				rp, err := root.Open(cfg.Context, s, key)
				if err != nil {
					return fmt.Errorf("opening %q: %w", key, err)
				}
				idx.Add(key)

				rf, err := rp.File(cfg.Context)
				if err != nil {
					return fmt.Errorf("opening %q: %w", rp.FileKey, err)
				}
				idx.Add(rp.FileKey)

				log.Printf("Scanning data reachable from %x...", rp.FileKey)
				start := time.Now()
				var numKeys int
				if err := rf.Scan(cfg.Context, func(key string, isFile bool) bool {
					numKeys++
					idx.Add(key)
					return true
				}); err != nil {
					return fmt.Errorf("scanning %q: %w", key, err)
				}
				log.Printf("Finished scanning %d blobs [%v elapsed]",
					numKeys, time.Since(start).Truncate(10*time.Millisecond))
			}

			// Sweep phase: Remove blobs not indexed.
			g, run := taskgroup.New(taskgroup.Trigger(cancel)).Limit(2 * runtime.NumCPU())

			log.Printf("Begin sweep over %d blobs...", n)
			start := time.Now()
			var numKeep, numDrop uint32
			for i := 0; i < 256; i++ {
				pfx := string([]byte{byte(i)})
				run(func() error {
					return s.List(cfg.Context, pfx, func(key string) error {
						if !strings.HasPrefix(key, pfx) {
							return blob.ErrStopListing
						} else if idx.Has(key) {
							atomic.AddUint32(&numKeep, 1)
							return nil
						}
						atomic.AddUint32(&numDrop, 1)
						return s.Delete(ctx, key)
					})
				})
			}
			log.Print("All key ranges listed, waiting for cleanup...")
			if err := g.Wait(); err != nil {
				return fmt.Errorf("sweeping failed: %w", err)
			}
			log.Printf("GC complete: keep %d, drop %d [%v elapsed]",
				numKeep, numDrop, time.Since(start).Truncate(10*time.Millisecond))
			return nil
		})
	},
}
package cmdgc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
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
			}
			idx := index.New(int(n), nil)

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
			g, run := taskgroup.New(taskgroup.Trigger(cancel)).Limit(runtime.NumCPU())

			log.Printf("Begin sweep over %d blobs...", n)
			start := time.Now()
			var numKeep, numDrop int
			if err := s.List(cfg.Context, "", func(key string) error {
				if idx.Has(key) {
					numKeep++
					return nil
				}
				numDrop++
				run(func() error { return s.Delete(ctx, key) })
				return nil
			}); err != nil {
				return err
			}
			log.Printf("Finished sweep: keep %d, drop %d [%v elapsed]; waiting for cleanup",
				numKeep, numDrop, time.Since(start).Truncate(10*time.Millisecond))
			if err := g.Wait(); err != nil {
				return fmt.Errorf("deleting unreachable blobs: %w", err)
			}
			log.Printf("GC complete [%v elapsed]", time.Since(start).Truncate(10*time.Millisecond))
			return nil
		})
	},
}

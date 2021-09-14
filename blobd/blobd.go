// Program blobd exports a blob.Store via JSON-RPC.
package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"flag"
	"fmt"
	"hash"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/creachadair/badgerstore"
	"github.com/creachadair/boltstore"
	"github.com/creachadair/ctrl"
	"github.com/creachadair/ffs/blob"
	"github.com/creachadair/ffs/blob/cachestore"
	"github.com/creachadair/ffs/blob/codecs/encrypted"
	"github.com/creachadair/ffs/blob/codecs/zlib"
	"github.com/creachadair/ffs/blob/encoded"
	"github.com/creachadair/ffs/blob/filestore"
	"github.com/creachadair/ffs/blob/memstore"
	"github.com/creachadair/ffs/blob/store"
	"github.com/creachadair/gcsstore"
	"github.com/creachadair/getpass"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/metrics"
	"github.com/creachadair/jrpc2/server"
	"github.com/creachadair/keyfile"
	"github.com/creachadair/leveldbstore"
	"github.com/creachadair/pebblestore"
	"github.com/creachadair/rpcstore"
	"github.com/creachadair/sqlitestore"
	"golang.org/x/crypto/sha3"
)

var (
	listenAddr = flag.String("listen", "", "Service address (required)")
	storeAddr  = flag.String("store", "", "Store address (required)")
	keyFile    = flag.String("keyfile", "", "Encryption key file")
	cacheSize  = flag.Int("cache", 0, "Memory cache size in KiB (0 means no cache)")
	doDebug    = flag.Bool("debug", false, "Enable server debug logging")
	zlibLevel  = flag.Int("zlib", 0, "Enable ZLIB compression (0 means no compression)")

	stores = store.Registry{
		"badger":  badgerstore.Opener,
		"bolt":    boltstore.Opener,
		"file":    filestore.Opener,
		"gcs":     gcsstore.Opener,
		"leveldb": leveldbstore.Opener,
		"memory":  memstore.Opener,
		"pebble":  pebblestore.Opener,
		"sqlite":  sqlitestore.Opener,
	}
)

func init() {
	var keys []string
	for key := range stores {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options] -store <spec> -listen <addr>

Start a JSON-RPC server that serves content from the blob.Store described by the -store
spec. The server listens at the specified address, which may be a host:port or the path
of a Unix-domain socket.

A store spec is a storage type and address: type:address
The types understood are: %[2]s

JSON-RPC requests are delimited by newlines.

With -keyfile, the store is opened with AES encryption.
Use -cache to enable a memory cache over the underlying store.

Options:
`, filepath.Base(os.Args[0]), strings.Join(keys, ", "))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	ctrl.Run(func() error {
		switch {
		case *listenAddr == "":
			ctrl.Exitf(1, "You must provide a non-empty -listen address")
		case *storeAddr == "":
			ctrl.Exitf(1, "You must provide a non-empty -store address")
		}

		ctx := context.Background()
		bs, hash := mustOpenStore(ctx)
		defer func() {
			if err := blob.CloseStore(ctx, bs); err != nil {
				log.Printf("Warning: closing store: %v", err)
			}
		}()
		log.Printf("Store address: %q", *storeAddr)
		if *zlibLevel > 0 {
			log.Printf("Compression enabled: ZLIB level %d", *zlibLevel)
			if *keyFile != "" {
				log.Printf(">> WARNING: Compression and encryption are both enabled")
			}
		}
		if *cacheSize > 0 {
			log.Printf("Memory cache size: %d KiB", *cacheSize)
		}
		if *keyFile != "" {
			log.Printf("Encryption key: %q", *keyFile)
		}

		svc := server.Static(
			rpcstore.NewService(bs, &rpcstore.ServiceOpts{Hash: hash}).Methods())

		lst, err := net.Listen(jrpc2.Network(*listenAddr))
		if err != nil {
			ctrl.Fatalf("Listen: %v", err)
		}
		if lst.Addr().Network() == "unix" {
			os.Chmod(*listenAddr, 0600) // best-effort
			defer os.Remove(*listenAddr)
		}
		log.Printf("Service: %q", *listenAddr)

		sig := make(chan os.Signal, 2)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			s, ok := <-sig
			if ok {
				log.Printf("Received signal: %v, closing listener", s)
				lst.Close()
				signal.Reset(syscall.SIGINT, syscall.SIGTERM)
			}
		}()

		mx := metrics.New()
		mx.SetLabel("blobd.store", *storeAddr)
		mx.SetLabel("blobd.pid", os.Getpid())
		mx.SetLabel("blobd.encrypted", *keyFile != "")
		mx.SetLabel("blobd.compressed", *zlibLevel > 0)
		mx.SetLabel("blobd.cacheSize", *cacheSize)

		var debug *log.Logger
		if *doDebug {
			debug = log.New(os.Stderr, "[blobd] ", log.LstdFlags)
		}
		if err := server.Loop(lst, svc, &server.LoopOptions{
			Framing: channel.Line,
			ServerOptions: &jrpc2.ServerOptions{
				Logger:    debug,
				Metrics:   mx,
				StartTime: time.Now().In(time.UTC),
			},
		}); err != nil {
			ctrl.Fatalf("Loop: %v", err)
		}
		return nil
	})
}

func mustOpenStore(ctx context.Context) (blob.Store, func() hash.Hash) {
	bs, err := stores.Open(ctx, *storeAddr)
	if err != nil {
		ctrl.Fatalf("Opening store: %v", err)
	}
	if *zlibLevel > 0 {
		bs = encoded.New(bs, zlib.NewCodec(zlib.Level(*zlibLevel)))
	}
	if *cacheSize > 0 {
		bs = cachestore.New(bs, *cacheSize<<10)
	}
	if *keyFile == "" {
		return bs, sha3.New256
	}

	key, err := keyfile.LoadKey(*keyFile, func() (string, error) {
		return getpass.Prompt("Passphrase: ")
	})
	if err != nil {
		ctrl.Fatalf("Loading encryption key: %v", err)
	}

	c, err := aes.NewCipher(key)
	if err != nil {
		ctrl.Fatalf("Creating cipher: %v", err)
	}
	gcm, err := cipher.NewGCM(c)
	if err != nil {
		ctrl.Fatalf("Creating GCM instance: %v", err)
	}
	return encoded.New(bs, encrypted.New(gcm, nil)), func() hash.Hash {
		return hmac.New(sha3.New256, key)
	}
}

// Program blobd exports a blob.Store via JSON-RPC.
package main

import (
	"context"
	"crypto/aes"
	"crypto/hmac"
	"flag"
	"fmt"
	"hash"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/creachadair/badgerstore"
	"github.com/creachadair/boltstore"
	"github.com/creachadair/ffs/blob"
	"github.com/creachadair/ffs/blob/cachestore"
	"github.com/creachadair/ffs/blob/codecs/encrypted"
	"github.com/creachadair/ffs/blob/encoded"
	"github.com/creachadair/ffs/blob/filestore"
	"github.com/creachadair/ffs/blob/rpcstore"
	"github.com/creachadair/ffs/blob/store"
	"github.com/creachadair/gcsstore"
	"github.com/creachadair/getpass"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/server"
	"github.com/creachadair/keyfile"
	"github.com/creachadair/sqlitestore"
	"golang.org/x/crypto/sha3"
)

var (
	listenAddr = flag.String("listen", "", "Service address (required)")
	storeAddr  = flag.String("store", "", "Store address (required)")
	keyFile    = flag.String("keyfile", "", "Encryption key file")
	cacheSize  = flag.Int("cache", 0, "Memory cache size in KiB (0 means no cache)")

	stores = store.Registry{
		"badger": badgerstore.Opener,
		"bolt":   boltstore.Opener,
		"file":   filestore.Opener,
		"gcs":    gcsstore.Opener,
		"sqlite": sqlitestore.Opener,
	}
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options] -store <spec> -listen <addr>

Start a JSON-RPC server that serves content from the blob.Store described by the -store
spec. The server listens at the specified address, which may be a host:port or the path
of a Unix-domain socket.

JSON-RPC requests are delimited by newlines.

With -keyfile, the store is opened with AES encryption.
Use -cache to enable a memory cache over the underlying store.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	switch {
	case *listenAddr == "":
		log.Fatal("You must provide a non-empty -listen address")
	case *storeAddr == "":
		log.Fatal("You must provide a non-empty -store address")
	}

	ctx := context.Background()
	bs, hash := mustOpenStore(ctx)
	defer func() {
		if err := blob.CloseStore(ctx, bs); err != nil {
			log.Printf("Warning: closing store: %v", err)
		}
	}()
	log.Printf("Store address: %q", *storeAddr)
	if *cacheSize > 0 {
		log.Printf("Memory cache size: %d KiB", *cacheSize)
	}
	if *keyFile != "" {
		log.Printf("Encryption key: %q", *keyFile)
	}

	svc := server.NewStatic(handler.NewService(
		rpcstore.NewService(bs, &rpcstore.ServiceOpts{Hash: hash})))

	ntype := jrpc2.Network(*listenAddr)
	lst, err := net.Listen(ntype, *listenAddr)
	if err != nil {
		log.Fatalf("Listen: %v", err)
	}
	if ntype == "unix" {
		os.Chmod(*listenAddr, 0600)
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

	if err := server.Loop(lst, svc, &server.LoopOptions{
		Framing: channel.Line,
	}); err != nil {
		log.Fatalf("Loop: %v", err)
	}
}

func mustOpenStore(ctx context.Context) (blob.Store, func() hash.Hash) {
	bs, err := stores.Open(ctx, *storeAddr)
	if err != nil {
		log.Fatalf("Opening store: %v", err)
	}
	if *cacheSize > 0 {
		bs = cachestore.New(bs, *cacheSize<<10)
	}
	if *keyFile == "" {
		return bs, sha3.New256
	}

	data, err := ioutil.ReadFile(*keyFile)
	if err != nil {
		log.Fatalf("Reading key file: %v", err)
	}
	kf, err := keyfile.Parse(data)
	if err != nil {
		log.Fatalf("Parsing key file: %v", err)
	}
	pp, err := getpass.Prompt("Passphrase: ")
	if err != nil {
		log.Fatalf("Reading passphrase: %v", err)
	}
	key, err := kf.Get(pp)
	if err != nil {
		log.Fatalf("Loading encryption key: %v", err)
	}

	c, err := aes.NewCipher(key)
	if err != nil {
		log.Fatalf("Creating cipher: %v", err)
	}
	return encoded.New(bs, encrypted.New(c, nil)), func() hash.Hash {
		return hmac.New(sha3.New256, key)
	}
}

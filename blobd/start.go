package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"hash"
	"log"
	"net"
	"os"

	"github.com/creachadair/ctrl"
	"github.com/creachadair/ffs/blob"
	"github.com/creachadair/ffs/blob/cachestore"
	"github.com/creachadair/ffs/blob/codecs/encrypted"
	"github.com/creachadair/ffs/blob/codecs/zlib"
	"github.com/creachadair/ffs/blob/encoded"
	"github.com/creachadair/getpass"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/server"
	"github.com/creachadair/keyfile"
	"github.com/creachadair/rpcstore"
	"golang.org/x/crypto/sha3"
)

type closer = func()

type startConfig struct {
	Store         blob.Store
	Address       string
	NewHash       func() hash.Hash
	ServerOptions *jrpc2.ServerOptions
}

func startNetServer(ctx context.Context, opts startConfig) (closer, <-chan error) {
	svc := server.Static(rpcstore.NewService(opts.Store, &rpcstore.ServiceOpts{
		Hash: opts.NewHash,
	}).Methods())

	lst, err := net.Listen(jrpc2.Network(opts.Address))
	if err != nil {
		ctrl.Fatalf("Listen: %v", err)
	}
	isUnix := lst.Addr().Network() == "unix"
	if isUnix {
		os.Chmod(opts.Address, 0600) // best-effort
	}
	log.Printf("Service: %q", opts.Address)
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		errc <- server.Loop(lst, svc, &server.LoopOptions{
			Framing:       channel.Line,
			ServerOptions: opts.ServerOptions,
		})
	}()

	return func() {
		lst.Close()
		if isUnix {
			defer os.Remove(opts.Address)
		}
	}, errc
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
		bs = cachestore.New(bs, *cacheSize<<20)
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

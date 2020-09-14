// Program blob provides basic support for reading and writing implementations
// of the blob.Store interface.
package main

import (
	"context"
	"crypto/aes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/creachadair/badgerstore"
	"github.com/creachadair/ffs/blob"
	"github.com/creachadair/ffs/blob/codecs/encrypted"
	"github.com/creachadair/ffs/blob/codecs/zlib"
	"github.com/creachadair/ffs/blob/encoded"
	"github.com/creachadair/ffs/blob/filestore"
	"github.com/creachadair/ffs/blob/store"
	"github.com/creachadair/getpass"
	"github.com/creachadair/keyfile"
	"github.com/creachadair/misctools/internal/vocab"
)

func init() {
	store.Default.Register("badger", badgerstore.Opener)
	store.Default.Register("file", filestore.Opener)
	store.Default.Register("zlib", func(ctx context.Context, addr string) (blob.Store, error) {
		s, err := store.Default.Open(ctx, addr)
		if err != nil {
			return nil, err
		}
		return encoded.New(s, zlib.NewCodec(zlib.LevelDefault)), nil
	})
}

func main() {
	v, err := vocab.New(filepath.Base(os.Args[0]), &tool{
		KeyFile: os.Getenv("KEYFILE_PATH"),
		CAS:     casGroup{Hash: "1"},
	})
	if err != nil {
		log.Fatalf("Setup failed: %v", err)
	}
	ctx := context.Background()
	if err := v.Dispatch(ctx, os.Args[1:]); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

type getCmd struct{}

func (getCmd) Run(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage is: get <key>")
	}
	key, err := parseKey(args[0])
	if err != nil {
		return err
	}
	bs, err := storeFromContext(ctx)
	if err != nil {
		return err
	}
	data, err := bs.Get(ctx, key)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

type putCmd struct {
	Replace bool `flag:"replace,Replace an existing key"`
}

func (p putCmd) Run(ctx context.Context, args []string) error {
	if len(args) == 0 || len(args) > 2 {
		return errors.New("usage is: put <key> [<path>]")
	}
	key, err := parseKey(args[0])
	if err != nil {
		return err
	}
	bs, err := storeFromContext(ctx)
	if err != nil {
		return nil
	}
	data, err := readData(ctx, "put", args[1:])
	if err != nil {
		return err
	}

	return bs.Put(ctx, blob.PutOptions{
		Key:     key,
		Data:    data,
		Replace: p.Replace,
	})
}

type sizeCmd struct{}

func (sizeCmd) Run(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage is: size <key>")
	}
	key, err := parseKey(args[0])
	if err != nil {
		return err
	}
	bs, err := storeFromContext(ctx)
	if err != nil {
		return err
	}
	size, err := bs.Size(ctx, key)
	if err != nil {
		return err
	}
	fmt.Println(size)
	return nil
}

type delCmd struct{}

func (delCmd) Run(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage is: delete <key>")
	}
	key, err := parseKey(args[0])
	if err != nil {
		return err
	}
	bs, err := storeFromContext(ctx)
	if err != nil {
		return err
	}
	del, ok := bs.(blob.Deleter)
	if !ok {
		return errors.New("store does not support deletion")
	}
	return del.Delete(ctx, key)
}

type listCmd struct {
	Raw   bool   `flag:"raw,Print raw keys without hex encoding"`
	Start string `flag:"start,List keys lexicographically greater than or equal to this"`
}

func (c listCmd) Run(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("usage is: list")
	}
	start, err := parseKey(c.Start)
	if err != nil {
		return err
	}
	bs, err := storeFromContext(ctx)
	if err != nil {
		return err
	}
	return bs.List(ctx, start, func(key string) error {
		if c.Raw {
			fmt.Println(key)
		} else {
			fmt.Printf("%x\n", key)
		}
		return nil
	})
}

type lenCmd struct{}

func (lenCmd) Run(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("usage is: len")
	}
	bs, err := storeFromContext(ctx)
	if err != nil {
		return err
	}
	n, err := bs.Len(ctx)
	if err != nil {
		return err
	}
	fmt.Println(n)
	return nil
}

type casGroup struct {
	Hash string `flag:"hash,CAS hash algorithm"`

	Key casKeyCmd `vocab:"key" help-summary:"Compute the key for a blob without writing it"`
	Put casPutCmd `vocab:"put" help-summary:"Write a content-addressed blob to the store"`
}

func (c *casGroup) Init(ctx context.Context, name string, args []string) (context.Context, error) {
	return context.WithValue(ctx, casKey{}, c), nil
}

func readData(ctx context.Context, cmd string, args []string) (data []byte, err error) {
	if len(args) == 0 {
		data, err = ioutil.ReadAll(os.Stdin)
	} else if len(args) == 1 {
		data, err = ioutil.ReadFile(args[0])
	} else {
		return nil, fmt.Errorf("usage is: %s [<path>]", cmd)
	}
	return
}

type casPutCmd struct{}

func (casPutCmd) Run(ctx context.Context, args []string) error {
	cas, err := casFromContext(ctx)
	if err != nil {
		return err
	}
	data, err := readData(ctx, "put", args)
	if err != nil {
		return err
	}
	key, err := cas.PutCAS(ctx, data)
	if err != nil {
		return err
	}
	fmt.Printf("%x\n", key)
	return nil
}

type casKeyCmd struct{}

func (casKeyCmd) Run(ctx context.Context, args []string) error {
	data, err := readData(ctx, "key", args)
	if err != nil {
		return err
	}
	h, err := hashFromContext(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("%x\n", blob.NewCAS(nil, h).Key(data))
	return nil
}

type tool struct {
	Addr    string `flag:"store,Blob store address (required)"`
	KeyFile string `flag:"keyfile,Path of encryption key file"`

	Get  getCmd  `vocab:"get" help-summary:"Read a blob from the store"`
	Put  putCmd  `vocab:"put" help-summary:"Write a blob to the store"`
	Size sizeCmd `vocab:"size" help-summary:"Print the size of a stored blob"`
	Del  delCmd  `vocab:"delete,del,rm" help-summary:"Delete a blob from the store"`
	List listCmd `vocab:"list,ls" help-summary:"List keys in the store"`
	Len  lenCmd  `vocab:"len,length" help-summary:"Print the number of stored keys"`

	CAS  casGroup   `vocab:"cas" help-summary:"Manipulate a content-addressable blob store"`
	Help vocab.Help `vocab:"help"`

	_ struct{} `help-summary:"Manipulate the contents of a blob store"`
	_ struct{} `help-long:"To specify blob keys literally, prefix them with @. To escape a leading @, double it.\nPrefix a base64-encoded key with \"+\". Otherwise, keys must be encoded in hexadecimal."`
}

func (t *tool) Init(ctx context.Context, name string, args []string) (context.Context, error) {
	return context.WithValue(ctx, toolKey{}, t), nil
}

type toolKey struct{}

func storeFromContext(ctx context.Context) (blob.Store, error) {
	t := ctx.Value(toolKey{}).(*tool)
	if t.Addr == "" {
		return nil, errors.New("no -store address was specified")
	}
	st, err := store.Default.Open(ctx, t.Addr)
	if err != nil {
		return nil, err
	}
	if t.KeyFile != "" {
		pp, err := getpass.Prompt("Passphrase: ")
		if err != nil {
			return nil, fmt.Errorf("reading passphrase: %v", err)
		}
		key, err := keyfile.LoadKey(t.KeyFile, pp)
		if err != nil {
			return nil, fmt.Errorf("loading encryption key: %v", err)
		}
		c, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("creating cipher: %v", err)
		}
		st = encoded.New(st, encrypted.New(c, nil))
	}
	return st, err
}

type casKey struct{}

func casFromContext(ctx context.Context) (blob.CAS, error) {
	h, err := hashFromContext(ctx)
	if err != nil {
		return blob.CAS{}, err
	}
	bs, err := storeFromContext(ctx)
	if err != nil {
		return blob.CAS{}, err
	}
	return blob.NewCAS(bs, h), nil
}

func hashFromContext(ctx context.Context) (func() hash.Hash, error) {
	c := ctx.Value(casKey{}).(*casGroup)
	switch c.Hash {
	case "1", "sha1":
		return sha1.New, nil
	case "2", "sha256":
		return sha256.New, nil
	case "":
		return nil, errors.New("hash not specified")
	default:
		return nil, fmt.Errorf("unknown hash algorithm %q", c.Hash)
	}
}

func parseKey(s string) (string, error) {
	if strings.HasPrefix(s, "@") {
		return s[1:], nil
	} else if strings.HasPrefix(s, "+") {
		key, err := base64.StdEncoding.DecodeString(s[1:])
		if err != nil {
			return "", fmt.Errorf("invalid key %q: %w", s, err)
		}
		return string(key), nil
	}
	key, err := hex.DecodeString(s)
	if err != nil {
		return "", fmt.Errorf("invalid key %q: %w", s, err)
	}
	return string(key), nil
}

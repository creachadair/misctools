// Program blob provides basic support for reading and writing implementations
// of the blob.Store interface.
package main

import (
	"context"
	"crypto/aes"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/creachadair/badgerstore"
	"github.com/creachadair/boltstore"
	"github.com/creachadair/command"
	"github.com/creachadair/ffs/blob"
	"github.com/creachadair/ffs/blob/codecs/encrypted"
	"github.com/creachadair/ffs/blob/codecs/zlib"
	"github.com/creachadair/ffs/blob/encoded"
	"github.com/creachadair/ffs/blob/filestore"
	"github.com/creachadair/ffs/blob/store"
	"github.com/creachadair/gcsstore"
	"github.com/creachadair/getpass"
	"github.com/creachadair/keyfile"
	"github.com/creachadair/sqlitestore"
	"golang.org/x/crypto/sha3"
)

var stores = store.Registry{
	"badger": badgerstore.Opener,
	"bolt":   boltstore.Opener,
	"file":   filestore.Opener,
	"gcs":    gcsstore.Opener,
	"sqlite": sqlitestore.Opener,
}

func main() {
	if err := command.Execute(tool.NewContext(nil), os.Args[1:]); err != nil {
		if errors.Is(err, command.ErrUsage) {
			os.Exit(2)
		}
		log.Fatalf("Error: %v", err)
	}
}

var getCmd = &command.C{
	Name:  "get",
	Usage: "get <key>...",
	Help:  "Read blobs from the store",

	Run: func(ctx *command.Context, args []string) error {
		if len(args) == 0 {
			//lint:ignore ST1005 The punctuation signifies repetition to the user.
			return errors.New("usage is: get <key>...")
		}
		bs, err := storeFromContext(ctx)
		if err != nil {
			return err
		}
		defer blob.CloseStore(getContext(ctx), bs)

		nctx := getContext(ctx)
		for _, arg := range args {
			key, err := parseKey(arg)
			if err != nil {
				return err
			}
			data, err := bs.Get(nctx, key)
			if err != nil {
				return err
			}
			os.Stdout.Write(data)
		}
		return nil
	},
}

func init() {
	putCmd.Flags.Bool("replace", false, "Replace an existing key")
}

var putCmd = &command.C{
	Name:  "put",
	Usage: "put <key> [<path>]",
	Help:  "Write a blob to the store",

	Run: func(ctx *command.Context, args []string) (err error) {
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
		defer func() {
			if cerr := blob.CloseStore(getContext(ctx), bs); err == nil {
				err = cerr
			}
		}()
		data, err := readData(getContext(ctx), "put", args[1:])
		if err != nil {
			return err
		}

		return bs.Put(getContext(ctx), blob.PutOptions{
			Key:     key,
			Data:    data,
			Replace: getFlag(ctx, "replace").(bool),
		})
	},
}

var sizeCmd = &command.C{
	Name:  "size",
	Usage: "size <key>...",
	Help:  "Print the sizes of stored blobs",

	Run: func(ctx *command.Context, args []string) error {
		if len(args) == 0 {
			//lint:ignore ST1005 The punctuation signifies repetition to the user.
			return errors.New("usage is: size <key>...")
		}
		bs, err := storeFromContext(ctx)
		if err != nil {
			return err
		}
		defer blob.CloseStore(getContext(ctx), bs)

		nctx := getContext(ctx)
		for _, arg := range args {
			key, err := parseKey(arg)
			if err != nil {
				return err
			}
			size, err := bs.Size(nctx, key)
			if err != nil {
				return err
			}
			fmt.Println(hex.EncodeToString([]byte(key)), size)
		}
		return nil
	},
}

var delCmd = &command.C{
	Name:  "delete",
	Usage: "delete <key>",
	Help:  "Delete a blob from the store",

	Run: func(ctx *command.Context, args []string) (err error) {
		if len(args) == 0 {
			//lint:ignore ST1005 The punctuation signifies repetition to the user.
			return errors.New("usage is: size <key>...")
		}
		bs, err := storeFromContext(ctx)
		if err != nil {
			return err
		}
		nctx := getContext(ctx)
		defer func() {
			if cerr := blob.CloseStore(nctx, bs); err == nil {
				err = cerr
			}
		}()
		for _, arg := range args {
			key, err := parseKey(arg)
			if err != nil {
				return err
			}
			if err := bs.Delete(nctx, key); errors.Is(err, blob.ErrKeyNotFound) {
				continue
			} else if err != nil {
				return err
			}
			fmt.Println(hex.EncodeToString([]byte(key)))
		}
		return nil
	},
}

func init() {
	listCmd.Flags.Bool("raw", false, "Print raw keys without hex encoding")
	listCmd.Flags.String("start", "", "List keys lexicographically greater than or equal to this")
}

var listCmd = &command.C{
	Name: "list",
	Help: "List keys in the store",

	Run: func(ctx *command.Context, args []string) error {
		if len(args) != 0 {
			return errors.New("usage is: list")
		}
		start, err := parseKey(getFlag(ctx, "start").(string))
		if err != nil {
			return err
		}
		bs, err := storeFromContext(ctx)
		if err != nil {
			return err
		}
		defer blob.CloseStore(getContext(ctx), bs)
		return bs.List(getContext(ctx), start, func(key string) error {
			if getFlag(ctx, "raw").(bool) {
				fmt.Println(key)
			} else {
				fmt.Printf("%x\n", key)
			}
			return nil
		})
	},
}

var lenCmd = &command.C{
	Name: "len",
	Help: "Print the number of stored keys",

	Run: func(ctx *command.Context, args []string) error {
		if len(args) != 0 {
			return errors.New("usage is: len")
		}
		bs, err := storeFromContext(ctx)
		if err != nil {
			return err
		}
		defer blob.CloseStore(getContext(ctx), bs)
		n, err := bs.Len(getContext(ctx))
		if err != nil {
			return err
		}
		fmt.Println(n)
		return nil
	},
}

var casGroup = &command.C{
	Name: "cas",
	Help: "Manipulate a content-addressable blob store",

	Commands: []*command.C{
		casKeyCmd,
		casPutCmd,
	},
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

var casPutCmd = &command.C{
	Name:  "put",
	Usage: "put",
	Help: `Write a content-addressed blob to the store.

The contents of the blob are read from stdin.`,

	Run: func(ctx *command.Context, args []string) (err error) {
		cas, err := casFromContext(ctx)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := blob.CloseStore(getContext(ctx), cas); err == nil {
				err = cerr
			}
		}()
		data, err := readData(getContext(ctx), "put", args)
		if err != nil {
			return err
		}
		key, err := cas.PutCAS(getContext(ctx), data)
		if err != nil {
			return err
		}
		fmt.Printf("%x\n", key)
		return nil
	},
}

var casKeyCmd = &command.C{
	Name: "key",
	Help: "Compute the key for a blob without writing it",

	Run: func(ctx *command.Context, args []string) error {
		data, err := readData(getContext(ctx), "key", args)
		if err != nil {
			return err
		}
		h, err := hashFromContext(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("%x\n", blob.NewCAS(nil, h).Key(data))
		return nil
	},
}

func init() {
	tool.Flags.String("store", "", "Blob store address (required)")
	tool.Flags.String("keyfile", os.Getenv("KEYFILE_PATH"), "Path of encryption key file")
	tool.Flags.String("hash", "3", "CAS hash algorithm (1, 2, 3)")
	tool.Flags.Int("zlib", 0, "ZLIB compression level (0=off)")
}

type settings struct {
	Context context.Context
	Keyfile string
	Store   string
	Hash    string
	Level   int

	newHash func() hash.Hash
}

var tool = &command.C{
	Name: filepath.Base(os.Args[0]),
	Usage: `[options] command [args...]
help [command]`,
	Help: `Manipulate the contents of a blob store.

To specify blob keys literally, prefix them with @.
To escape a leading @, double it.
Prefix a base64-encoded key with "+".
Otherwise, keys must be encoded in hexadecimal.
`,

	Init: func(ctx *command.Context) error {
		ctx.Config = &settings{
			Context: context.Background(),
			Keyfile: getFlag(ctx, "keyfile").(string),
			Store:   getFlag(ctx, "store").(string),
			Hash:    getFlag(ctx, "hash").(string),
			Level:   getFlag(ctx, "zlib").(int),
		}
		return nil
	},

	Commands: []*command.C{
		getCmd, putCmd, sizeCmd, delCmd, listCmd, lenCmd, casGroup,
		{
			Name:  "help",
			Usage: "[topic/command]",
			Help:  "Print help for the specified command or topic",

			CustomFlags: true,
			Run:         command.RunHelp,
		},
	},
}

func storeFromContext(ctx *command.Context) (blob.Store, error) {
	t := ctx.Config.(*settings)
	if t.Store == "" {
		return nil, errors.New("no -store address was specified")
	}
	st, err := stores.Open(t.Context, t.Store)
	if err != nil {
		return nil, err
	}
	if t.Level > 0 {
		st = encoded.New(st, zlib.NewCodec(zlib.Level(t.Level)))
	}
	if t.Keyfile != "" {
		h, err := hashFromContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting hash: %w", err)
		}
		pp, err := getpass.Prompt("Passphrase: ")
		if err != nil {
			return nil, fmt.Errorf("reading passphrase: %v", err)
		}
		key, err := keyfile.LoadKey(t.Keyfile, pp)
		if err != nil {
			return nil, fmt.Errorf("loading encryption key: %v", err)
		}
		c, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("creating cipher: %v", err)
		}
		st = encoded.New(st, encrypted.New(c, nil))
		t.newHash = func() hash.Hash {
			return hmac.New(h, key)
		}
	}
	return st, err
}

func casFromContext(ctx *command.Context) (blob.CAS, error) {
	bs, err := storeFromContext(ctx)
	if err != nil {
		return blob.CAS{}, err
	}
	h, err := hashFromContext(ctx)
	if err != nil {
		return blob.CAS{}, err
	}
	return blob.NewCAS(bs, h), nil
}

func hashFromContext(ctx *command.Context) (func() hash.Hash, error) {
	c := ctx.Config.(*settings)
	if c.newHash != nil {
		return c.newHash, nil
	}
	switch c.Hash {
	case "1", "sha1":
		return sha1.New, nil
	case "2", "sha256":
		return sha256.New, nil
	case "3", "sha3":
		return sha3.New256, nil
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

func getFlag(ctx *command.Context, name string) interface{} {
	v := ctx.Command.Flags.Lookup(name).Value
	return v.(flag.Getter).Get()
}

func getContext(ctx *command.Context) context.Context {
	return ctx.Config.(*settings).Context
}

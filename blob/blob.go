// Program blob provides basic support for reading and writing implementations
// of the blob.Store interface.
package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/creachadair/command"
	"github.com/creachadair/ffs/blob"
	"github.com/creachadair/ffs/blob/rpcstore"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
)

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
	listCmd.Flags.String("prefix", "", "List only keys having this prefix")
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
		pfx, err := parseKey(getFlag(ctx, "prefix").(string))
		if err != nil {
			return err
		}
		if pfx != "" && start == "" {
			start = pfx
		}
		bs, err := storeFromContext(ctx)
		if err != nil {
			return err
		}
		defer blob.CloseStore(getContext(ctx), bs)

		return bs.List(getContext(ctx), start, func(key string) error {
			if !strings.HasPrefix(key, pfx) {
				if key > pfx {
					return blob.ErrStopListing
				}
				return nil
			} else if getFlag(ctx, "raw").(bool) {
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
		cas, err := storeFromContext(ctx)
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
		cas, err := storeFromContext(ctx)
		if err != nil {
			return err
		}
		data, err := readData(getContext(ctx), "key", args)
		if err != nil {
			return err
		}
		key, err := cas.Key(getContext(ctx), data)
		if err != nil {
			return err
		}
		fmt.Printf("%x\n", key)
		return nil
	},
}

func init() {
	tool.Flags.String("store", os.Getenv("BLOB_STORE"), "Blob store address (required)")
	tool.Flags.Bool("debug", false, "Enable client debug logging")
}

type settings struct {
	Context context.Context
	Store   string
	Debug   bool
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

The BLOB_STORE environment variable is read to choose a default
blob store address; otherwise -store must be set.

`,

	Init: func(ctx *command.Context) error {
		store := os.ExpandEnv(getFlag(ctx, "store").(string))
		ctx.Config = &settings{
			Context: context.Background(),
			Store:   store,
			Debug:   getFlag(ctx, "debug").(bool),
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

func storeFromContext(ctx *command.Context) (rpcstore.Store, error) {
	t := ctx.Config.(*settings)
	if t.Store == "" {
		return rpcstore.Store{}, errors.New("no -store address was specified")
	}
	conn, err := net.Dial(jrpc2.Network(t.Store), t.Store)
	if err != nil {
		return rpcstore.Store{}, fmt.Errorf("dialing: %w", err)
	}
	var logger *log.Logger
	if t.Debug {
		logger = log.New(os.Stderr, "[client] ", log.LstdFlags)
	}
	cli := jrpc2.NewClient(channel.Line(conn, conn), &jrpc2.ClientOptions{
		Logger: logger,
	})
	return rpcstore.NewClient(cli, nil), nil
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

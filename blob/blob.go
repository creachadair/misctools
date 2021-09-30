// Program blob provides basic support for reading and writing implementations
// of the blob.Store interface.
package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
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
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/rpcstore"
)

func main() {
	if err := command.Execute(tool.NewEnv(nil), os.Args[1:]); err != nil {
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

	Run: func(env *command.Env, args []string) error {
		if len(args) == 0 {
			//lint:ignore ST1005 The punctuation signifies repetition to the user.
			return errors.New("usage is: get <key>...")
		}
		bs, err := storeFromEnv(env)
		if err != nil {
			return err
		}
		defer blob.CloseStore(getContext(env), bs)

		nctx := getContext(env)
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

	Run: func(env *command.Env, args []string) (err error) {
		if len(args) == 0 || len(args) > 2 {
			return errors.New("usage is: put <key> [<path>]")
		}
		key, err := parseKey(args[0])
		if err != nil {
			return err
		}
		bs, err := storeFromEnv(env)
		if err != nil {
			return nil
		}
		defer func() {
			if cerr := blob.CloseStore(getContext(env), bs); err == nil {
				err = cerr
			}
		}()
		data, err := readData(getContext(env), "put", args[1:])
		if err != nil {
			return err
		}

		return bs.Put(getContext(env), blob.PutOptions{
			Key:     key,
			Data:    data,
			Replace: getFlag(env, "replace").(bool),
		})
	},
}

var sizeCmd = &command.C{
	Name:  "size",
	Usage: "size <key>...",
	Help:  "Print the sizes of stored blobs",

	Run: func(env *command.Env, args []string) error {
		if len(args) == 0 {
			//lint:ignore ST1005 The punctuation signifies repetition to the user.
			return errors.New("usage is: size <key>...")
		}
		bs, err := storeFromEnv(env)
		if err != nil {
			return err
		}
		defer blob.CloseStore(getContext(env), bs)

		nctx := getContext(env)
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

func init() {
	delCmd.Flags.Bool("missing-ok", false, "Do not report an error for missing keys")
}

var delCmd = &command.C{
	Name:  "delete",
	Usage: "delete <key>",
	Help:  "Delete a blob from the store",

	Run: func(env *command.Env, args []string) (err error) {
		if len(args) == 0 {
			//lint:ignore ST1005 The punctuation signifies repetition to the user.
			return errors.New("usage is: delete <key>...")
		}
		bs, err := storeFromEnv(env)
		if err != nil {
			return err
		}
		nctx := getContext(env)
		defer func() {
			if cerr := blob.CloseStore(nctx, bs); err == nil {
				err = cerr
			}
		}()
		missingOK := getFlag(env, "missing-ok").(bool)
		for _, arg := range args {
			key, err := parseKey(arg)
			if err != nil {
				return err
			}
			if err := bs.Delete(nctx, key); blob.IsKeyNotFound(err) && missingOK {
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

	Run: func(env *command.Env, args []string) error {
		if len(args) != 0 {
			return errors.New("usage is: list")
		}
		start, err := parseKey(getFlag(env, "start").(string))
		if err != nil {
			return err
		}
		pfx, err := parseKey(getFlag(env, "prefix").(string))
		if err != nil {
			return err
		}
		if pfx != "" && start == "" {
			start = pfx
		}
		bs, err := storeFromEnv(env)
		if err != nil {
			return err
		}
		defer blob.CloseStore(getContext(env), bs)

		return bs.List(getContext(env), start, func(key string) error {
			if !strings.HasPrefix(key, pfx) {
				if key > pfx {
					return blob.ErrStopListing
				}
				return nil
			} else if getFlag(env, "raw").(bool) {
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

	Run: func(env *command.Env, args []string) error {
		if len(args) != 0 {
			return errors.New("usage is: len")
		}
		bs, err := storeFromEnv(env)
		if err != nil {
			return err
		}
		defer blob.CloseStore(getContext(env), bs)
		n, err := bs.Len(getContext(env))
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

	Run: func(env *command.Env, args []string) (err error) {
		cas, err := storeFromEnv(env)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := blob.CloseStore(getContext(env), cas); err == nil {
				err = cerr
			}
		}()
		data, err := readData(getContext(env), "put", args)
		if err != nil {
			return err
		}
		key, err := cas.PutCAS(getContext(env), data)
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

	Run: func(env *command.Env, args []string) error {
		cas, err := storeFromEnv(env)
		if err != nil {
			return err
		}
		data, err := readData(getContext(env), "key", args)
		if err != nil {
			return err
		}
		key, err := cas.Key(getContext(env), data)
		if err != nil {
			return err
		}
		fmt.Printf("%x\n", key)
		return nil
	},
}

var statCmd = &command.C{
	Name: "status",
	Help: "Print blob server status",

	Run: func(env *command.Env, args []string) error {
		s, err := storeFromEnv(env)
		if err != nil {
			return err
		}
		si, err := s.ServerInfo(getContext(env))
		if err != nil {
			return err
		}
		msg, err := json.Marshal(si)
		if err != nil {
			return err
		}
		fmt.Println(string(msg))
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

Since blob keys are usually binary, key arguments are assumed to be encoded.

Rule                                                     Example
- To specify blob keys literally, prefix them with "@"   @foo
  To escape a leading @, double it                       @@foo
- If the key is all hex digits, decode it as hex         666f6f0a
- Otherwise, it is treated as base64.                    Zm9vCg==

The BLOB_STORE environment variable is read to choose a default store address;
otherwise -store must be set.

`,

	Init: func(env *command.Env) error {
		store := os.ExpandEnv(getFlag(env, "store").(string))
		env.Config = &settings{
			Context: context.Background(),
			Store:   store,
			Debug:   getFlag(env, "debug").(bool),
		}
		return nil
	},

	Commands: []*command.C{
		getCmd, putCmd, sizeCmd, delCmd, listCmd, lenCmd, casGroup, statCmd,
		{
			Name:  "help",
			Usage: "[topic/command]",
			Help:  "Print help for the specified command or topic",

			CustomFlags: true,
			Run:         command.RunHelp,
		},
	},
}

func storeFromEnv(env *command.Env) (rpcstore.Store, error) {
	t := env.Config.(*settings)
	if t.Store == "" {
		return rpcstore.Store{}, errors.New("no -store address was specified")
	}
	conn, err := net.Dial(jrpc2.Network(t.Store))
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
	}
	var key []byte
	var err error
	if isAllHex(s) {
		key, err = hex.DecodeString(s)
	} else if strings.HasSuffix(s, "=") {
		key, err = base64.StdEncoding.DecodeString(s)
	} else {
		key, err = base64.RawStdEncoding.DecodeString(s) // tolerate missing padding
	}
	if err != nil {
		return "", fmt.Errorf("invalid key %q: %w", s, err)
	}
	return string(key), nil
}

func getFlag(env *command.Env, name string) interface{} {
	v := env.Command.Flags.Lookup(name).Value
	return v.(flag.Getter).Get()
}

func getContext(env *command.Env) context.Context {
	return env.Config.(*settings).Context
}

func isAllHex(s string) bool {
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

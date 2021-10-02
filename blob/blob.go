// Program blob provides basic support for reading and writing implementations
// of the blob.Store interface.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/creachadair/command"
)

type settings struct {
	Context context.Context
	Store   string
	Debug   bool
}

func main() {
	if err := command.Execute(tool.NewEnv(nil), os.Args[1:]); err != nil {
		if errors.Is(err, command.ErrUsage) {
			os.Exit(2)
		}
		log.Fatalf("Error: %v", err)
	}
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

	SetFlags: func(env *command.Env, fs *flag.FlagSet) {
		fs.String("store", os.Getenv("BLOB_STORE"), "Blob store address (required)")
		fs.Bool("debug", false, "Enable client debug logging")
	},

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
		{
			Name:  "get",
			Usage: "get <key>...",
			Help:  "Read blobs from the store",
			Run:   getCmd,
		},
		{
			Name:  "put",
			Usage: "put <key> [<path>]",
			Help:  "Write a blob to the store",

			SetFlags: func(env *command.Env, fs *flag.FlagSet) {
				fs.Bool("replace", false, "Replace an existing key")
			},
			Run: putCmd,
		},
		{
			Name:  "size",
			Usage: "size <key>...",
			Help:  "Print the sizes of stored blobs",
			Run:   sizeCmd,
		},
		{
			Name:  "delete",
			Usage: "delete <key>",
			Help:  "Delete a blob from the store",

			SetFlags: func(env *command.Env, fs *flag.FlagSet) {
				fs.Bool("missing-ok", false, "Do not report an error for missing keys")
			},
			Run: delCmd,
		},
		{
			Name: "list",
			Help: "List keys in the store",

			SetFlags: func(env *command.Env, fs *flag.FlagSet) {
				fs.Bool("raw", false, "Print raw keys without hex encoding")
				fs.String("start", "", "List keys lexicographically greater than or equal to this")
				fs.String("prefix", "", "List only keys having this prefix")
			},
			Run: listCmd,
		},
		{
			Name: "len",
			Help: "Print the number of stored keys",
			Run:  lenCmd,
		},
		{
			Name: "cas",
			Help: "Manipulate a content-addressable blob store",

			Commands: []*command.C{
				{
					Name: "key",
					Help: "Compute the key for a blob without writing it",
					Run:  casKeyCmd,
				},
				{
					Name:  "put",
					Usage: "put",
					Help:  "Write a content-addressed blob to the store from stdin.",
					Run:   casPutCmd,
				},
			},
		},
		{
			Name: "status",
			Help: "Print blob server status",
			Run:  statCmd,
		},
		command.HelpCommand(nil),
	},
}

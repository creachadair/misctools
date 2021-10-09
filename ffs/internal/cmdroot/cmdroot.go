package cmdroot

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/creachadair/command"
	"github.com/creachadair/ffs/blob"
	"github.com/creachadair/ffs/file"
	"github.com/creachadair/ffs/file/root"
	"github.com/creachadair/ffs/file/wiretype"
	"github.com/creachadair/misctools/ffs/config"
)

var Command = &command.C{
	Name: "root",
	Help: "Manipulate filesystem root pointers",

	Commands: []*command.C{
		{
			Name:  "view",
			Usage: "<root-key>",
			Help:  "Print the representation of a filesystem root",

			Run: runView,
		},
		{
			Name: "list",
			Help: "List the root keys known in the store",

			Run: runList,
		},
		{
			Name:  "create",
			Usage: "<name> <description>...",
			Help:  "Create a new empty root pointer",

			SetFlags: func(_ *command.Env, fs *flag.FlagSet) {
				fs.BoolVar(&createFlags.Replace, "replace", false, "Replace an existing root name")
			},
			Run: runCreate,
		},
	},
}

func runView(env *command.Env, args []string) error {
	keys, err := config.RootKeys(args)
	if err != nil {
		return err
	} else if len(keys) == 0 {
		return errors.New("missing required <root-key>")
	}

	cfg := env.Config.(*config.Settings)
	return cfg.WithStore(cfg.Context, func(s blob.CAS) error {
		rp, err := root.Open(cfg.Context, s, keys[0])
		if err != nil {
			return err
		}
		msg := root.Encode(rp).Value.(*wiretype.Object_Root).Root
		fmt.Println(config.ToJSON(msg))
		return nil
	})
}

func runList(env *command.Env, args []string) error {
	if len(args) != 0 {
		return errors.New("extra arguments after command")
	}
	cfg := env.Config.(*config.Settings)
	return cfg.WithStore(cfg.Context, func(s blob.CAS) error {
		return s.List(cfg.Context, "root:", func(key string) error {
			if !strings.HasPrefix(key, "root:") {
				return blob.ErrStopListing
			}
			fmt.Println(key)
			return nil
		})
	})
}

var createFlags struct {
	Replace bool
}

func runCreate(env *command.Env, args []string) error {
	if len(args) < 2 {
		return errors.New("usage is: <name> <description>...") //lint:ignore ST1005 User message.
	}
	key := config.RootKey(args[0])
	desc := strings.Join(args[1:], " ")
	cfg := env.Config.(*config.Settings)
	return cfg.WithStore(cfg.Context, func(s blob.CAS) error {
		fk, err := file.New(s, &file.NewOptions{
			Stat: &file.Stat{Mode: os.ModeDir | 0755},
		}).Flush(cfg.Context)
		if err != nil {
			return fmt.Errorf("creating new file: %w", err)
		}
		return root.New(s, &root.Options{
			Description: desc,
			FileKey:     fk,
		}).Save(cfg.Context, key, createFlags.Replace)
	})
}

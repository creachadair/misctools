package cmdroot

import (
	"errors"
	"fmt"
	"strings"

	"github.com/creachadair/command"
	"github.com/creachadair/ffs/blob"
	"github.com/creachadair/ffs/file/wiretype"
	"github.com/creachadair/misctools/ffs/config"
	"google.golang.org/protobuf/proto"
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
		bits, err := s.Get(cfg.Context, keys[0])
		if err != nil {
			return err
		}
		var obj wiretype.Object
		if err := proto.Unmarshal(bits, &obj); err != nil {
			return err
		}
		rp, ok := obj.Value.(*wiretype.Object_Root)
		if !ok {
			return fmt.Errorf("wrong object type %T", obj.Value)
		}
		fmt.Println(config.ToJSON(rp.Root))
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

package cmdfile

import (
	"errors"
	"fmt"
	"strings"

	"github.com/creachadair/command"
	"github.com/creachadair/ffs/blob"
	"github.com/creachadair/ffs/file"
	"github.com/creachadair/ffs/file/root"
	"github.com/creachadair/ffs/file/wiretype"
	"github.com/creachadair/ffs/fpath"
	"github.com/creachadair/misctools/ffs/config"
)

var Command = &command.C{
	Name: "file",
	Help: "Manipulate file and directory objects",

	Commands: []*command.C{
		{
			Name: "show",
			Usage: `@<root-key> [path]
<file-key> [path]`,
			Help: "Print the representation of a filesystem root",

			Run: runShow,
		},
	},
}

func runShow(env *command.Env, args []string) error {
	if len(args) == 0 {
		return errors.New("missing required storage key")
	}
	cfg := env.Config.(*config.Settings)
	return cfg.WithStore(cfg.Context, func(s blob.CAS) error {
		var fileKey string
		if strings.HasPrefix(args[0], "@") {
			rp, err := root.Open(cfg.Context, s, config.RootKey(args[0][1:]))
			if err != nil {
				return err
			}
			fileKey = rp.FileKey
		} else if fk, err := config.ParseKey(args[0]); err != nil {
			return err
		} else {
			fileKey = fk
		}

		fp, err := file.Open(cfg.Context, s, fileKey)
		if err != nil {
			return err
		}
		if len(args) > 1 {
			tp, err := fpath.Open(cfg.Context, fp, args[1])
			if err != nil {
				return err
			}
			fileKey, _ = tp.Flush(cfg.Context)
			fp = tp
		}
		msg := file.Encode(fp).Value.(*wiretype.Object_Node).Node
		fmt.Println(config.ToJSON(map[string]interface{}{
			"fileKey": []byte(fileKey),
			"node":    msg,
		}))
		return nil
	})
}

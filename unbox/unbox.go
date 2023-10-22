// Program unbox lists and unpacks e-mail messages from a Unix mbox file.
package main

import (
	"os"
	"path/filepath"

	"github.com/creachadair/command"
)

func main() {
	root := &command.C{
		Name:  filepath.Base(os.Args[0]),
		Usage: "command [args]\nhelp [command",
		Help:  `Unpack e-mail messages from mbox format.`,
		Commands: []*command.C{
			{
				Name:  "list",
				Usage: "<mailbox>",
				Help:  "List the mail messages in the specified mailbox.",
				Run:   command.Adapt(runList),
			},
			{
				Name:  "burst",
				Usage: "<mailbox> <output-dir>",
				Help:  "Burst e-mail messages into individual files.",
				Run:   command.Adapt(runBurst),
			},
			command.HelpCommand(nil),
			command.VersionCommand(),
		},
	}
	command.RunOrFail(root.NewEnv(nil).MergeFlags(true), os.Args[1:])
}

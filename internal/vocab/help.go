// Copyright (C) 2019 Michael J. Fromberger. All Rights Reserved.

package vocab

import (
	"context"
	"strings"

	"golang.org/x/xerrors"
)

// The Helper interface may be optionally implemented by a command to generate
// long help text.
type Helper interface {
	// Help returns long help text for the receiver.
	Help() string
}

// The Summarizer interface may be optionally implemented by a command to
// generate summary help text.
type Summarizer interface {
	// Summary returns the summary help text for the receiver.
	Summary() string
}

// Help implements a generic "help" subcommand that prints out the full help
// text from the command described by its arguments.
//
// An instance of Help may be embedded into a command struct to provide the
// help subcommand.
type Help struct {
	_ struct{} `help-summary:"Show help for a command"`
	_ struct{} `help-long:"Use 'help <command>' or '<command> <subcommand> help' for help with\na specific command."`
}

// Run implements the "help" subcommand.
func (Help) Run(ctx context.Context, args []string) error {
	target, tail := parentItem(ctx).Resolve(args)
	if len(tail) != 0 {
		return xerrors.Errorf("help: unable to resolve %q", strings.Join(args, " "))
	}
	target.longHelp()
	return nil
}

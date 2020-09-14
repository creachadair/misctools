// Copyright (C) 2019 Michael J. Fromberger. All Rights Reserved.

package vocab_test

import (
	"context"
	"io/ioutil"
	"testing"

	"bitbucket.org/creachadair/shell"
	"github.com/creachadair/misctools/internal/vocab"
)

type doNothing struct{}

func (doNothing) Run(context.Context, []string) error { return nil }

func TestNewErrors(t *testing.T) {
	tests := []interface{}{
		// Various non-struct types.
		nil,
		1,
		"foo",
		'\u3030',
		[]string{"apple", "pear", "plum"},

		// A struct type with no behaviour.
		struct{}{},
		&struct{}{},

		// Unable to flag fields of non-pointer structs.
		struct {
			A doNothing `vocab:"a"`
			F int       `flag:"f,whatever"`
		}{},

		// Unable to flag fields on a nil struct pointer.
		struct {
			Q *struct {
				B doNothing `vocab:"b"`
				F int       `flag:"f,whatever"`
			} `vocab:"q"`
		}{},

		// Unable to flag fields that are themselves nil.
		&struct {
			F *int `flag:"bad,Ro ma ro ma ma"`
		}{},

		// Unable to flag values that do not implement flag.Value.
		&struct {
			A doNothing `vocab:"a"`
			F []string  `flag:"nope,Nope nope nope"`
		}{},

		// Unable to attach vocabulary to unexported fields.
		&struct {
			doNothing `vocab:"nope,sorry"`
		}{},

		// Duplicate subcommand names.
		struct {
			A doNothing `vocab:"a"`
			B doNothing `vocab:"a"`
		}{},

		// Duplicate alias.
		struct {
			A doNothing `vocab:"a,c"`
			B doNothing `vocab:"b,c"`
		}{},
	}
	for _, test := range tests {
		got, err := vocab.New("test", test)
		if err == nil {
			t.Errorf("New(%+v): got %+v, want error", test, got)
		} else {
			t.Logf("New(%+v) correctly failed: %v", test, err)
		}
	}
}

func TestNew(t *testing.T) {
	logfn := func(s string) vocab.RunFunc {
		return func(ctx context.Context, args []string) error {
			t.Logf("[%s] %d args %q", s, len(args), args)
			return nil
		}
	}
	v := struct {
		A vocab.RunFunc `vocab:"a" help-long:"The A test command"`
		B vocab.RunFunc `vocab:"b" help-summary:"Command B" help-long:"The long version of B"`
		C vocab.RunFunc // not annotated
		D int           `flag:"d,Number of elements"`

		H vocab.Help `vocab:"help"`

		_ struct{} `help-summary:"A command to test the dispatcher"`
		_ struct{} `help-long:"This command verifies that various dispatch rules are obeyed for\ncommand lines expected to be well-formed."`
	}{
		A: logfn("alpha"),
		B: logfn("bravo"),
		C: func(_ context.Context, args []string) error {
			t.Error("Function C should not have been called")
			return nil
		},
		D: 25,
	}
	itm, err := vocab.New("test", &v)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	itm.SetOutput(ioutil.Discard)

	ctx := context.Background()
	for _, cmd := range []string{
		// Empty arguments generate short help.
		"",

		// Run the help command for the root.
		"help",

		// Run the help command for a subcommand.
		"help a",
		"help b",

		// Run commands, with and without arguments.
		"a",
		`a "is for amy" who fell "down the" stairs`,
		"b",
		`b "is for basil" "assaulted" by 'bears'`,

		// Verify that flags get parsed along the way.
		`-d 22 a foo`,
	} {
		args, _ := shell.Split(cmd)
		t.Logf("Dispatch args: %q", args)
		if err := itm.Dispatch(ctx, args); err != nil {
			t.Errorf("Dispatch %q: unexpected error: %v", args, err)
		}
	}
}

package vocab_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"bitbucket.org/creachadair/shell"
	"github.com/creachadair/misctools/internal/vocab"
)

func Example() {
	v, err := vocab.New("tool", &tool{
		WorkDir: "/tmp",
	})
	if err != nil {
		log.Fatalf("New failed: %v", err)
	}
	v.SetOutput(os.Stdout) // default is os.Stderr

	ctx := context.Background()
	for _, cmd := range []string{
		"",             // simple help, short form
		"help",         // global help, long form
		"help file",    // group help
		"help file ls", // subcommand help
		"file list",    // execute

		"-workdir /var/log file list", // global flags
		"file list -a",                // local flags

		`text echo "all your base" are "belong to us"`,
	} {
		args, _ := shell.Split(cmd)
		fmt.Printf("------ %q\n", args)
		if err := v.Dispatch(ctx, args); err != nil {
			log.Fatalf("ERROR: %q: %v", cmd, err)
		}
	}
	// Output:
	// ------ []
	// tool: A trivial command-line shell
	//
	// Subcommands:
	//   file   Subcommands pertaining to files (alias: files)
	//   help   Show help for a command (alias: wtf)
	//   text   Subcommands pertaining to text
	// ------ ["help"]
	// tool: A trivial command-line shell
	//
	// Demonstrate the use of the vocab package by wiring up
	// a simple command-line with nested commands.
	//
	// Flags:
	//   -workdir string
	//     	Set working directory to this path (default "/tmp")
	//
	// Subcommands:
	//   file   Subcommands pertaining to files (alias: files)
	//   help   Show help for a command (alias: wtf)
	//   text   Subcommands pertaining to text
	// ------ ["help" "file"]
	// file: Subcommands pertaining to files
	//
	// Subcommands:
	//   ls   List file metadata (alias: dir, list)
	// ------ ["help" "file" "ls"]
	// ls: List file metadata
	//
	// Flags:
	//   -a	List all files, including dotfiles
	// ------ ["file" "list"]
	// src
	// docs
	// ------ ["-workdir" "/var/log" "file" "list"]
	// src
	// docs
	// ------ ["file" "list" "-a"]
	// .config
	// src
	// docs
	// ------ ["text" "echo" "all your base" "are" "belong to us"]
	// all your base are belong to us
}

type list struct {
	All bool `flag:"a,List all files, including dotfiles"`
}

func (ls list) Run(ctx context.Context, args []string) error {
	if ls.All {
		fmt.Println(".config")
	}
	fmt.Println("src")
	fmt.Println("docs")
	return nil
}

type echo struct{}

func (echo) Run(ctx context.Context, args []string) error {
	fmt.Println(strings.Join(args, " "))
	return nil
}

type tool struct {
	WorkDir string `flag:"workdir,Set working directory to this path"`

	// tool files ls <path> ...
	Files struct {
		List list `vocab:"ls,list,dir" help-summary:"List file metadata"`

		_ struct{} `help-summary:"Subcommands pertaining to files"`
	} `vocab:"file,files"`

	// tool text echo <args> ...
	Text struct {
		Echo echo `vocab:"echo,print,type" help-summary:"Concatenate and print arguments"`

		_ struct{} `help-summary:"Subcommands pertaining to text"`
	} `vocab:"text"`

	// tool help [command]
	Help vocab.Help `vocab:"help,wtf"`

	_ struct{} `help-summary:"A trivial command-line shell"`
	_ struct{} `help-long:"Demonstrate the use of the vocab package by wiring up\na simple command-line with nested commands."`
}

var _ vocab.Initializer = (*tool)(nil)

// Init sets up the context for subcommands run via t. In this case, it ensures
// the specified working directory exists and that $PWD points to it.
func (t tool) Init(ctx context.Context, name string, args []string) (context.Context, error) {
	if err := os.MkdirAll(t.WorkDir, 0755); err != nil {
		return nil, err
	}
	return ctx, os.Chdir(t.WorkDir)
}

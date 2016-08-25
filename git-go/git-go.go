// Program git-go is a Git plugin that adds some helpful behaviour for
// repositories containing Go code.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

var out = os.Stdout

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: git go [subcommand] [options]

Helpful additions for writing and maintaining Go code.

Subcommands:
  presubmit   : run "go test" and "go vet" over all packages
`)
		flag.PrintDefaults()
	}
}
func main() {
	if err := run(); err != nil {
		log.Fatal("Error: ", err)
	}
}

func run() error {
	flag.Parse()
	if flag.NArg() == 0 {
		return errors.New("no subcommand specified")
	}

	switch flag.Arg(0) {
	case "presubmit":
		root, err := rootDir()
		if err != nil {
			return err
		}
		test := invoke(runTests(root))
		lint := invoke(runLinter(root))
		if test != nil {
			return test
		} else if lint != nil {
			return lint
		}
	case "help":
		flag.Usage()
	default:
		return fmt.Errorf("subcommand %q not understood", flag.Arg(0))
	}
	return nil
}

func rootDir() (string, error) {
	data, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	return strings.TrimSpace(string(data)), err
}

func gocmd(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Dir = dir
	return cmd
}

func runTests(path string) *exec.Cmd { return gocmd(path, "test", "./...") }

func runLinter(path string) *exec.Cmd { return gocmd(path, "vet", "./...") }

func invoke(cmd *exec.Cmd) error {
	fmt.Fprintf(out, "▷ \033[1;36m%s\033[0m\n", strings.Join(cmd.Args, " "))
	err := cmd.Run()
	if err == nil {
		fmt.Fprintln(out, "\033[50C\033[1;32mPASSED\033[0m")
	} else {
		fmt.Fprintln(out, "\033[50C\033[1;31mFAILED\033[0m")
	}
	return err
}

// Program git-go is a Git plugin that adds some helpful behaviour for
// repositories containing Go code.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var out = os.Stdout

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: git go [subcommand] [options]

Helpful additions for writing and maintaining Go code.

Subcommands:
  presubmit      : run "go test" and "go vet" over all packages
  test, tests    : run "go test" over all packages
  lint           : run "go vet" over all packages
  install-hook   : install pre-push hook in the current repo
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
	case "test", "tests":
		root, err := rootDir()
		if err != nil {
			return err
		}
		return invoke(runTests(root))

	case "lint":
		root, err := rootDir()
		if err != nil {
			return err
		}
		return invoke(runLinter(root))

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

	case "install-hook":
		root, err := rootDir()
		if err != nil {
			return err
		}
		hookdir := filepath.Join(root, ".git", "hooks")
		prepush := filepath.Join(hookdir, "pre-push")
		if _, err := os.Stat(prepush); os.IsNotExist(err) {
			return writeHook(prepush)
		} else if err == nil {
			return fmt.Errorf("pre-push hook already exists")
		} else {
			return err
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

func writeHook(path string) error {
	const content = `#!/bin/sh
#
# Verify that the code is in a useful state before pushing.
git go presubmit
`
	return ioutil.WriteFile(path, []byte(content), 0755)
}

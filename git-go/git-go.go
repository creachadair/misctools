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
		fmt.Fprint(os.Stderr, `Usage: git go [subcommand] [options]

Helpful additions for writing and maintaining Go code.

Subcommands:
  presubmit      : run "go test" and "go vet" over all packages
  test, tests    : run "go test" over all packages
  vet            : run "go vet" over all packages
  lint           : run "golint" over all packages (if installed)
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
	} else if flag.Arg(0) == "install-hook" {
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
	} else if flag.Arg(0) == "help" {
		flag.Usage()
		return nil
	}

	root, err := rootDir()
	if err != nil {
		return err
	}
	var nerr int
	for _, arg := range flag.Args() {
		err := func() error {

			switch arg {
			case "test", "tests":
				return invoke(runTests(root))

			case "vet":
				return invoke(runVet(root))

			case "lint":
				return invoke(runLint(root))

			case "presubmit":
				test := invoke(runTests(root))
				vet := invoke(runVet(root))
				if test != nil {
					return test
				} else if vet != nil {
					return vet
				}

			default:
				return fmt.Errorf("subcommand %q not understood", arg)
			}
			return nil
		}()
		if err != nil {
			log.Printf("Error: %v", err)
			nerr++
		}
	}
	if nerr > 0 {
		return fmt.Errorf("%d problems found", nerr)
	}
	return nil
}

func rootDir() (string, error) {
	data, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	return strings.TrimSpace(string(data)), err
}

func gocmd(dir string, args ...string) *exec.Cmd { return runcmd("go", dir, args...) }

func runcmd(bin, dir string, args ...string) *exec.Cmd {
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Dir = dir
	return cmd
}

func runTests(path string) *exec.Cmd { return gocmd(path, "test", "./...") }

func runVet(path string) *exec.Cmd { return gocmd(path, "vet", "./...") }

func runLint(path string) *exec.Cmd { return runcmd("golint", path, "-set_exit_status", "./...") }

func invoke(cmd *exec.Cmd) error {
	fmt.Fprintf(out, "▷ \033[1;36m%s\033[0m\n", strings.Join(cmd.Args, " "))
	err := cmd.Run()
	switch t := err.(type) {
	case nil:
		fmt.Fprintln(out, "\033[50C\033[1;32mPASSED\033[0m")
	case *exec.Error:
		fmt.Fprintf(out, "\033[50C\033[1;33mSKIPPED\033[0m (%v)\n", t)
		return nil
	default:
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

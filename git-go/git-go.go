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
  presubmit    : run "gofmt", "go test", and "go vet" over all packages
  test, test   : run "go test" over all packages
  vet          : run "go vet" over all packages
  lint         : run "golint" over all packages (if installed)
  fmt, format  : run "gofmt -s" over all packages (if installed)
  check        : run all the above checks

  install-hook [subcommand]
               : install pre-push hook in the current repo.
                 subcommand defaults to "presubmit"

Set GITGO_LINT=warn to convert lint failures into warnings.
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
		subcommand := "presubmit"
		if flag.NArg() > 1 {
			subcommand = flag.Arg(1)
		}
		hookdir := filepath.Join(root, ".git", "hooks")
		prepush := filepath.Join(hookdir, "pre-push")
		if _, err := os.Stat(prepush); os.IsNotExist(err) {
			return writeHook(prepush, subcommand)
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
	args := flag.Args()
	if len(args) == 1 && args[0] == "check" {
		args = []string{"format", "test", "vet", "lint"}
	}
	var nerr int
	for _, arg := range args {
		err := func() error {
			switch arg {
			case "test", "tests":
				return invoke(runTests(root))

			case "vet":
				return invoke(runVet(root))

			case "lint":
				lint := invoke(runLint(root))
				if _, ok := lint.(*exec.ExitError); ok && lintMode() == "warn" {
					fmt.Fprintln(out, "\t[NOTE] \033[1;33mIgnoring linter failure "+
						"because lint mode is \"warn\"\033[0m")
					return nil
				}
				return lint

			case "presubmit":
				fumpt := invoke(runFumpt(root))
				test := invoke(runTests(root))
				vet := invoke(runVet(root))
				if fumpt != nil {
					return fumpt
				} else if test != nil {
					return test
				} else if vet != nil {
					return vet
				}

			case "fmt", "format":
				return invoke(runFumpt(root))

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

func runTests(path string) *exec.Cmd { return gocmd(path, "test", "-race", "-cpu=1,2", "./...") }

func runVet(path string) *exec.Cmd { return gocmd(path, "vet", "./...") }

func runLint(path string) *exec.Cmd { return runcmd("golint", path, "-set_exit_status", "./...") }

func runFumpt(path string) *exec.Cmd {
	const script = `find . -type f -name '*.go' -print0 \
| xargs -0 gofmt -l -s \
| grep .
if [ $? -eq 0 ] ; then
  echo "^ These files need formatting with go fmt"
  exit 1
fi
`

	cmd := exec.Command("/bin/sh", "-s", "--", "gofmt", "-l", "-s", "./...")
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Dir = path
	return cmd
}

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

func writeHook(path, subcommand string) error {
	content := fmt.Sprintf(`#!/bin/sh
#
# Verify that the code is in a useful state before pushing.
git go %s
`, subcommand)

	return ioutil.WriteFile(path, []byte(content), 0755)
}

func lintMode() string {
	if m := os.Getenv("GITGO_LINT"); m != "" {
		return m
	}
	return "error"
}

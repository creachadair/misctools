// Program hublink generates URLs to files stored in GitHub.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/creachadair/command"
)

var (
	useBranch string // default: current branch
	lineSpan  string // default: link to the file
	doBrowse  bool   // open in the browser
)

const (
	githubBase = "https://github.com/"
)

func main() {
	env := (&command.C{
		Name:  filepath.Base(os.Args[0]),
		Usage: "<command> [arguments]",
		Help:  "A command-line to link to objects in GitHub",

		SetFlags: func(env *command.Env, _ *flag.FlagSet) {
			for _, cmd := range env.Command.Commands {
				setStdFlags(&cmd.Flags)
			}
		},

		Init: func(*command.Env) error {
			if useBranch == "" {
				cur, err := currentBranch()
				if err != nil {
					return fmt.Errorf("finding current branch: %v", err)
				}
				useBranch = cur
			}
			return nil
		},

		Commands: []*command.C{
			{
				Name:  "file",
				Usage: "<path>",
				Help:  "Generate a link to the specified repository file",

				Run: func(env *command.Env, args []string) error {
					if len(args) == 0 {
						return errors.New("no paths specified")
					}
					repo, dir, err := repoNameRoot()
					if err != nil {
						return err
					}
					lo, hi, err := parseLineSpan(lineSpan)
					if err != nil {
						return fmt.Errorf("invalid line span: %v", err)
					}
					for _, path := range args {
						real, err := fixPath(dir, path)
						if err != nil {
							return fmt.Errorf("invalid path: %v", err)
						}
						var buf bytes.Buffer
						buf.WriteString(githubBase)
						buf.WriteString(repo)
						buf.WriteString("/blob/" + useBranch + "/")
						buf.WriteString(real)

						if lo > 0 {
							fmt.Fprintf(&buf, "#L%d", lo)
							if hi > lo {
								fmt.Fprintf(&buf, "-%d", hi)
							}
						}
						if err := printOrOpen(buf.String()); err != nil {
							return nil
						}
					}
					return nil
				},
			},
			command.HelpCommand(nil),
		},
	}).NewEnv(nil)
	if err := command.Execute(env, os.Args[1:]); err == command.ErrUsage {
		os.Exit(2)
	} else if err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func currentBranch() (string, error) { return git("branch", "--show-current") }

func firstRemote() (string, error) {
	rems, err := git("remote", "show", "-n")
	if err != nil {
		return "", fmt.Errorf("listing remotes: %v", err)
	}
	return strings.SplitN(rems, "\n", 2)[0], nil
}

func repoNameRoot() (name, dir string, _ error) {
	dir, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", fmt.Errorf("getting repository root: %v", err)
	}
	remote, err := firstRemote()
	if err != nil {
		return "", "", err
	}
	url, err := git("remote", "get-url", remote)
	if err != nil {
		return "", "", fmt.Errorf("getting remote URL: %v", err)
	}
	if strings.HasPrefix(url, "git@") {
		return strings.TrimSuffix(strings.TrimPrefix(url, "git@github.com:"), ".git"), dir, nil
	}
	return strings.TrimSuffix(strings.TrimPrefix(url, githubBase), ".git"), dir, nil
}

func git(cmd string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{cmd}, args...)...).Output()
	if err != nil {
		var ex *exec.ExitError
		if errors.As(err, &ex) {
			return "", errors.New(strings.SplitN(string(ex.Stderr), "\n", 2)[0])
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func fixPath(dir, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Rel(dir, abs)
}

func parseLineSpan(s string) (lo, hi int, err error) {
	if s == "" {
		return 0, 0, nil
	}
	parts := strings.SplitN(s, "-", 2)
	lo, err = strconv.Atoi(parts[0])
	if err == nil && len(parts) == 2 {
		hi, err = strconv.Atoi(parts[1])
	}
	return lo, hi, err
}

func printOrOpen(s string) error {
	if doBrowse {
		return exec.Command("open", s).Run()
	}
	fmt.Println(s)
	return nil
}

func setStdFlags(fs *flag.FlagSet) {
	fs.StringVar(&useBranch, "b", "", "Link to this branch (default is current)")
	fs.BoolVar(&doBrowse, "open", false, "Open link in browser")
	fs.StringVar(&lineSpan, "line", "", "Specify a line span to refer to")
}

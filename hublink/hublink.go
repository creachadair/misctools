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
	"regexp"
	"strconv"
	"strings"

	"github.com/creachadair/command"
)

var (
	useBranch string // default: current branch
	useHash   bool   // 	use the commit hash rather than the branch name
	doBrowse  bool   // open in the browser

	spanRE = regexp.MustCompile(`:(\d+)(?:-(\d+))?$`)
)

const (
	githubBase = "https://github.com/"
)

func main() {
	env := (&command.C{
		Name:  filepath.Base(os.Args[0]),
		Usage: "<command> [arguments]",
		Help:  "A command-line to link to objects in GitHub",

		SetFlags: func(env *command.Env, fs *flag.FlagSet) {
			fs.BoolVar(&doBrowse, "open", false, "Open link in browser")
			for _, cmd := range env.Command.Commands {
				setStdFlags(&cmd.Flags)
			}
		},

		Commands: []*command.C{
			{
				Name: "file",
				Usage: `
<path>        -- link to entire file
<path>:LINE   -- link to specific line    
<path>:LO-HI  -- link to a range of lines (LO < HI)`,
				Help: `Generate a link to the specified repository files.

By default, a link is generated for the current branch.
The repository name is derived from the first remote.`,

				Run: func(env *command.Env, args []string) error {
					if len(args) == 0 {
						return errors.New("no paths specified")
					}
					repo, dir, err := repoNameRoot()
					if err != nil {
						return err
					}
					target, err := resolveBranch(useBranch)
					if err != nil {
						return fmt.Errorf("resolving branch: %v", err)
					}

					for _, raw := range args {
						path, lo, hi, err := parseFile(raw)
						if err != nil {
							return fmt.Errorf("invalid file: %v", err)
						}
						real, err := fixPath(dir, path)
						if err != nil {
							return fmt.Errorf("invalid path: %v", err)
						}
						var buf bytes.Buffer
						buf.WriteString(githubBase)
						buf.WriteString(repo)
						buf.WriteString("/blob/" + target + "/")
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

func resolveBranch(branch string) (string, error) {
	if branch == "" {
		cur, err := currentBranch()
		if err != nil {
			return "", fmt.Errorf("finding current branch: %v", err)
		}
		branch = cur
	}
	if useHash {
		return git("rev-parse", branch)
	}
	return branch, nil
}

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

func parseFile(s string) (path string, lo, hi int, err error) {
	m := spanRE.FindStringSubmatchIndex(s)
	if m == nil {
		return s, 0, 0, nil // no span indicators
	}
	path = s[:m[0]]
	lo, err = strconv.Atoi(s[m[2]:m[3]])
	if err == nil && m[4] >= 0 {
		hi, err = strconv.Atoi(s[m[4]:m[5]])
	}
	return path, lo, hi, err
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
	fs.BoolVar(&useHash, "H", false, "Use commit hash instead of branch name")
}

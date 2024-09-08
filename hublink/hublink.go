// Program hublink generates URLs to files stored in GitHub.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
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
	useHash   bool   // use the commit hash rather than the branch name
	doBrowse  bool   // open in the browser
	doRaw     bool   // link to raw file content
)

const (
	githubBase    = "https://github.com/"
	githubRawBase = "https://raw.githubusercontent.com/"
)

func main() {
	env := (&command.C{
		Name:  filepath.Base(os.Args[0]),
		Usage: "<command> [arguments]",
		Help:  "A command-line to link to objects in GitHub",

		SetFlags: func(env *command.Env, fs *flag.FlagSet) {
			fs.BoolVar(&doBrowse, "open", false, "Open link in browser")
			setStdFlags(fs)
			for _, cmd := range env.Command.Commands {
				setStdFlags(&cmd.Flags)
			}
		},

		Run: runLinkFile,

		Commands: []*command.C{
			{
				Name: "file",
				Usage: `
<path>       -- link to entire file
<path>:LINE  -- link to a single specific line
<path>:LO-HI -- link to a range of lines (LO < HI)
<path>:@RE   -- link to the first match of this regexp (RE2)
`,
				Help: `Generate a link to the specified repository files.

By default, a link is generated for the current branch.
The repository name is derived from the first remote.`,

				Run: runLinkFile,
			},
			{
				Name:  "grep",
				Usage: "[git-grep-flags] <pattern> [<path>...]",
				Help:  "Generate a link to a git grep match.",

				CustomFlags: true,
				Run:         runGrepFile,
			},
			command.HelpCommand(nil),
		},
	}).NewEnv(nil).MergeFlags(false)
	command.RunOrFail(env, os.Args[1:])
}

func addFlag(flag string, args []string) []string {
	for _, arg := range args {
		if arg == flag {
			return args
		}
	}
	return append([]string{flag}, args...)
}

func runGrepFile(env *command.Env) error {
	out, err := git("grep", addFlag("-n", env.Args)...)
	if err != nil {
		return fmt.Errorf("no matches: %w", err)
	}
	hits := strings.Split(strings.TrimSpace(out), "\n")
	if len(hits) != 1 {
		fmt.Println(out)
		return fmt.Errorf("found %d matches", len(hits))
	}
	parts := strings.SplitN(hits[0], ":", 3)
	target := strings.Join(parts[:2], ":")
	env.Args = append(env.Args, target)
	return runLinkFile(env)
}

func runLinkFile(env *command.Env) error {
	if len(env.Args) == 0 {
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

	for _, raw := range env.Args {
		path, lo, hi, err := parseFile(raw)
		if err != nil {
			return fmt.Errorf("invalid file spec: %v", err)
		}
		real, err := fixPath(dir, path)
		if err != nil {
			return fmt.Errorf("invalid path: %v", err)
		}

		var buf bytes.Buffer
		if doRaw {
			buf.WriteString(githubRawBase)
			buf.WriteString(repo)
			fmt.Fprintf(&buf, "/%s/", target)
			buf.WriteString(real)
		} else {
			buf.WriteString(githubBase)
			buf.WriteString(repo)
			fmt.Fprintf(&buf, "/%s/%s/", gitObjectType(real), target)
			buf.WriteString(real)

			if lo > 0 {
				fmt.Fprintf(&buf, "#L%d", lo)
				if hi > lo {
					fmt.Fprintf(&buf, "-L%d", hi)
				}
			}
		}
		if err := printAndOpen(buf.String()); err != nil {
			return nil
		}
	}
	return nil
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

// parseFile parses a file path with an optional coda specifying a location
// within the file. The coda may have the following forms
//
//	:dd
//	:dd-dd
//	:@re
//
// where "dd" represents decimal digits and "re" represents an RE2 regular
// expression. The first two forms indicate a single line or range of lines,
// the third indicates the line or range corresponding to the first match of
// the given regular expression in the file content.
func parseFile(s string) (path string, lo, hi int, err error) {
	if i := strings.LastIndex(s, ":"); i < 0 {
		return s, 0, 0, nil
	} else {
		path, s = s[:i], s[i+1:]
	}
	if strings.HasPrefix(s, "@") {
		re, err := regexp.Compile("(?msU)" + s[1:])
		if err != nil {
			return path, 0, 0, fmt.Errorf("invalid regexp: %w", err)
		}
		return grepFile(path, re)
	}
	span := strings.SplitN(s, "-", 2)
	lo, err = strconv.Atoi(span[0])
	if err == nil && len(span) == 2 {
		hi, err = strconv.Atoi(span[1])
	}
	return path, lo, hi, err
}

func grepFile(path string, re *regexp.Regexp) (_ string, lo, hi int, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return path, 0, 0, err
	}
	m := re.FindIndex(data)
	if m == nil {
		return path, 0, 0, fmt.Errorf("no match for regexp %v", re)
	}
	lo = bytes.Count(data[:m[0]], []byte("\n")) + 1
	hi = lo + bytes.Count(data[m[0]:m[1]], []byte("\n"))
	return path, lo, hi, nil
}

func gitObjectType(path string) string {
	fi, err := os.Stat(path)
	if err == nil && fi.IsDir() {
		return "tree"
	}
	return "blob"
}

func printAndOpen(s string) error {
	fmt.Println(s)
	if doBrowse {
		return exec.Command("open", s).Run()
	}
	return nil
}

func setStdFlags(fs *flag.FlagSet) {
	fs.StringVar(&useBranch, "b", "", "Link to this branch (default is current)")
	fs.BoolVar(&useHash, "H", false, "Use commit hash instead of branch name")
	fs.BoolVar(&doRaw, "raw", false, "Link to raw file content (ignores offsets)")
}

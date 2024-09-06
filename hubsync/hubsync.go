// Program hubsync pulls changes from a remote to the default branch
// of a local clone of a GitHub repository, then rebases local branches onto
// that branch if they have a corresponding remote branch.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/creachadair/mds/mapset"
	"github.com/creachadair/mds/shell"
)

var (
	defBranch    = flag.String("base", "", `Base branch name (if "", use default from remote)`)
	useRemote    = flag.String("remote", "origin", "Use this remote name")
	skipBranches = flag.String("skip", "", "Branches to skip during update (comma-separated)")
	workFile     = flag.String("worklist", "hubsync.json", "Work list save file")
	doForcePush  = flag.Bool("push", false, "Force-push updated branches to remote")
	doResume     = flag.Bool("resume", false, "Resume from an existing work list")
	doVerbose    = flag.Bool("v", false, "Verbose logging")
	doDebug      = flag.Bool("debug", false, "Enable debug mode")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [flags]

Rebase those branches in a local copy of a GitHub repository that are based off
the default (main) branch in the remote. To do this, the tool:

- Identifies the base branch. If -base is set, that name is used; otherwise
  the tool looks up the default branch from the remote.

- Makes a work list of local branches descended from the base branch.

- Switches to the base branch and executes "git pull" to update from remote.

- Switches to each branch in the work list and rebases it onto the updated
  base branch. After doing this, if the local branch tracks a remote branch,
  and -push is true, it executes a "git push -f" to update the remote copy.

  Use -skip to give a comma-separated list of branch names to skip updating.

- Switches back to the original branch from which the tool was run.

In case of an error during rebase, the work list is saved at the root of the
repository in a file named "hubsync.json" (use -worklist to change this),
and the update stops. After fixing any problems (such as merge conflicts),
use %[1]s -resume to pick up from where the update left off. To start over,
move or delete the worklist file.

This tool only works inside a Git repository.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	if *useRemote == "" {
		log.Fatal("You must specify a -remote name to use")
	}
	skip := parseSkips(*skipBranches)

	// Set working directory to the repository root.
	root, err := repoRoot()
	if err != nil {
		log.Fatalf("Repository root: %v", err)
	} else if err := os.Chdir(root); err != nil {
		log.Fatalf("Chdir: %v", err)
	}

	work, err := openWorkList(*workFile)
	if err != nil {
		log.Fatalf("Loading worklist: %v", err)
	} else if work.Loaded && !*doResume {
		log.Fatalf(`Found non-empty worklist. Remove %q and re-run or pass -resume to continue`,
			*workFile)
	} else if !work.Loaded && *doResume {
		log.Fatalf("There is no worklist file to -resume from")
	}
	defer work.resetDir()

	// Pull the latest content. Note we need to do this after checking branches,
	// since it changes which branches follow the default.
	if !work.Loaded {
		// Save the worklist before pulling the base branch, since a successful
		// pull will break the ancestry relationship. It's preserved in the list.
		if err := work.saveTo(*workFile); err != nil {
			log.Fatalf("Saving initial worklist: %v", err)
		}
		log.Printf("Saved worklist with %d branches to %q", len(work.Branches), *workFile)

		log.Printf("Pulling base branch %q", work.Base)
		if err := pullBranch(work.Base); err != nil {
			log.Fatalf("Pull %q failed: %v", work.Base, err)
		}
	}

	// Bail out if no branches need updating.
	if nu := work.numUnfinished(); nu == 0 {
		log.Print("No branches require update")
		cleanupWorklist(work)
		return
	} else if work.Loaded {
		log.Printf("Resuming update onto branch %q (%d branches remaining to update)", work.Base, nu)
	}

	// Rebase the local branches onto the default, and if requested and
	// necessary, push the results back up to the remote.
	for _, br := range work.Branches {
		if br.Done {
			log.Printf("Skipping branch %q (already updated)", br.Name)
			continue
		} else if skip.Has(br.Name) {
			log.Printf("Skipping branch %q (per -skip)", br.Name)
			continue
		}

		log.Printf("Rebasing %q onto %q", br.Name, work.Base)
		if _, err := git("rebase", work.Base, br.Name); err != nil {
			log.Fatalf("Rebase failed: %v", err)
		}
		if !work.Push || br.Remote == "" {
			// nothing to do
		} else if ok, err := forcePush(*useRemote, br.Name); err != nil {
			log.Fatalf("Updating %q: %v", br.Name, err)
		} else if ok {
			log.Printf("- Forced update of %q to %s", br.Name, br.Remote)
		}
		br.Done = true
		if err := work.saveTo(*workFile); err != nil {
			log.Fatalf("Saving worklist: %v", err)
		}
	}
	cleanupWorklist(work)
}

func currentBranch() (string, error) { return git("branch", "--show-current") }

func forcePush(remote, branch string) (bool, error) {
	bits, err := exec.Command("git", "push", "-f", remote, branch).CombinedOutput()
	msg := string(bits)
	if err != nil {
		return false, errors.New(strings.SplitN(msg, "\n", 2)[0])
	}
	ok := strings.ToLower(strings.TrimSpace(msg)) != "everything up-to-date"
	return ok, nil
}

type branchInfo struct {
	Name   string `json:"local"`
	Remote string `json:"remote,omitempty"`
	Done   bool   `json:"done"`
}

func listBranchInfo(matching, dbranch, useRemote string) ([]*branchInfo, error) {
	var out []*branchInfo

	listOut, err := git("branch", "--list",
		"--contains", dbranch,
		"--format", "%(refname:short)\t%(upstream:short)",
		matching,
	)
	if err != nil {
		return nil, err
	}
	list := strings.Split(strings.TrimSpace(listOut), "\n")
	for _, s := range list {
		parts := strings.SplitN(s, "\t", 2)
		if parts[0] == dbranch {
			continue // skip the default
		} else if len(parts) == 1 {
			parts = append(parts, "")
		}
		out = append(out, &branchInfo{
			Name:   parts[0],
			Remote: parts[1],
		})
	}
	return out, nil
}

func pullBranch(branch string) error {
	if _, err := git("checkout", branch); err != nil {
		return fmt.Errorf("checkout: %w", err)
	}
	_, err := git("pull")
	return err
}

func repoRoot() (string, error) { return git("rev-parse", "--show-toplevel") }

func defaultBranch(defBranch, useRemote string) (string, error) {
	if defBranch != "" {
		return defBranch, nil
	}
	rem, err := git("remote", "show", useRemote)
	if err != nil {
		return "", err
	}
	const needle = "HEAD branch:"
	for _, line := range strings.Split(rem, "\n") {
		i := strings.Index(line, needle)
		if i >= 0 {
			return strings.TrimSpace(line[i+len(needle):]), nil
		}
	}
	return "", errors.New("default branch not found")
}

func git(cmd string, args ...string) (string, error) {
	if *doVerbose {
		log.Println("[git]", cmd, shell.Join(args))
	}
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

func cleanupWorklist(work *workList) {
	if *doDebug {
		log.Printf("Worklist left intact at %q (per -debug)", *workFile)
	} else if err := os.Remove(*workFile); os.IsNotExist(err) {
		// OK
	} else if err != nil {
		log.Printf("Warning: removing worklist: %v", err)
	}
}

func parseSkips(skip string) mapset.Set[string] {
	if skip == "" {
		return nil
	}
	return mapset.New(strings.Split(skip, ",")...)
}

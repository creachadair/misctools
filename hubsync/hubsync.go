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
	"strings"

	"bitbucket.org/creachadair/shell"
)

var (
	defBranch    = flag.String("base", "", `Base branch name (if "", use default from remote)`)
	branchPrefix = flag.String("prefix", "", "Select branches matching this prefix")
	useRemote    = flag.String("remote", "origin", "Use this remote name")
	skipBranches = flag.String("skip", "", "Branches to skip during update (comma-separated)")
	workFile     = flag.String("worklist", "hubsync.json", "Work list save file")
	noForcePush  = flag.Bool("nopush", false, "Skip force-pushing updated branches to remote")
	doResume     = flag.Bool("resume", false, "Resume from an existing work list")
	doVerbose    = flag.Bool("v", false, "Verbose logging")
	doDebug      = flag.Bool("debug", false, "Enable debug mode")
)

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
		log.Printf("Pulling base branch %q", work.Base)
		if err := pullBranch(work.Base); err != nil {
			log.Fatalf("Pull %q: %v", work.Base, err)
		}
	}

	// Bail out if no branches need updating.
	if nu := work.numUnfinished(); nu == 0 {
		log.Print("No branches require update")
		return
	} else if work.Loaded {
		log.Printf("Resuming update onto branch %q (%d branches remaining to update)", work.Base, nu)
	}

	// Rebase the local branches onto the default, and if requested and
	// necessary, push the results back up to the remote.
	if !work.Loaded {
		if err := work.saveTo(*workFile); err != nil {
			log.Fatalf("Saving initial worklist: %v", err)
		}
		log.Printf("Saved worklist with %d branches to %q", len(work.Branches), *workFile)
	}
	for _, br := range work.Branches {
		if br.Done {
			log.Printf("Skipping branch %q (already updated)", br.Name)
			continue
		} else if skip[br.Name] {
			log.Printf("Skipping branch %q (per -skip)", br.Name)
			continue
		}

		log.Printf("Rebasing %q onto %q", br.Name, work.Base)
		if _, err := git("rebase", work.Base, br.Name); err != nil {
			log.Fatalf("Rebase failed: %v", err)
		}
		if *noForcePush || !br.Remote {
			// nothing to do
		} else if ok, err := forcePush(*useRemote, br.Name); err != nil {
			log.Fatalf("Updating %q: %v", br.Name, err)
		} else if ok {
			log.Printf("- Forced update of %q to %s", br.Name, *useRemote)
		}
		br.Done = true
		if err := work.saveTo(*workFile); err != nil {
			log.Fatalf("Saving worklist: %v", err)
		}
	}

	// If we successfully get here, the worklist is clean.
	if work.Loaded && !*doDebug {
		if err := os.Remove(*workFile); err != nil {
			log.Printf("Warning: removing worklist: %v", err)
		}
	}
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
	Name   string `json:"name"`
	Remote bool   `json:"hasRemote"`
	Done   bool   `json:"done"`
}

func listBranchInfo(matching, dbranch, useRemote string) ([]*branchInfo, error) {
	var out []*branchInfo

	localOut, err := git("branch", "--list", "--contains", dbranch, matching)
	if err != nil {
		return nil, err
	}
	local := strings.Split(strings.TrimSpace(localOut), "\n")
	for _, s := range local {
		clean := strings.TrimPrefix(strings.TrimSpace(s), "* ")
		if clean == "" || clean == dbranch {
			continue
		}
		out = append(out, &branchInfo{Name: clean})
	}

	remoteOut, err := git("branch", "--list", "-r", useRemote+"/"+matching)
	if err != nil {
		return nil, err
	}
	remote := strings.Split(strings.TrimSpace(remoteOut), "\n")
	for i, s := range remote {
		remote[i] = strings.TrimPrefix(strings.TrimSpace(s), useRemote+"/")
	}

	for _, lb := range out {
		if lb.Name == dbranch {
			continue // don't consider the base branch regardless
		}
		for _, rb := range remote {
			if rb == lb.Name {
				lb.Remote = true
				break
			}
		}
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

func parseSkips(skip string) map[string]bool {
	if skip == "" {
		return nil
	}
	m := make(map[string]bool)
	for _, name := range strings.Split(skip, ",") {
		m[name] = true
	}
	return m
}

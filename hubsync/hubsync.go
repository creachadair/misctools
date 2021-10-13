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
)

var (
	useRemote    = flag.String("remote", "origin", "Use this remote name")
	branchPrefix = flag.String("prefix", "", "Select branches matching this prefix")
	doForcePush  = flag.Bool("push", false, "Force push updated branches to remote")
)

func main() {
	flag.Parse()
	switch {
	case *useRemote == "":
		log.Fatal("You must specify a -remote name to use")
	case *branchPrefix == "":
		log.Fatal("You must specify a branch -prefix")
	}

	// Set working directory to the repository root.
	root, err := repoRoot()
	if err != nil {
		log.Fatalf("Repository root: %v", err)
	} else if err := os.Chdir(root); err != nil {
		log.Fatalf("Chdir: %v", err)
	}

	// Save the name of the current branch so we can go back.
	save, err := currentBranch()
	if err != nil {
		log.Fatalf("Current branch: %v", err)
	}
	defer git("checkout", save)

	// Find the name of the default branch.
	dbranch, err := defaultBranch(*useRemote)
	if err != nil {
		log.Fatalf("Default branch: %v", err)
	}
	log.Printf("Pulling default branch %q", dbranch)
	if err := pullBranch(dbranch); err != nil {
		log.Fatalf("Pull %q: %v", dbranch, err)
	}

	// List local branches that track corresponding remote branches.
	rem, err := branchesWithRemotes(*branchPrefix+"*", dbranch, *useRemote)
	if err != nil {
		log.Fatalf("Listing branches: %v", err)
	}
	if len(rem) == 0 {
		log.Print("No branches require update")
	}

	// Rebase the local branches onto the default, and
	for _, br := range rem {
		log.Printf("Rebasing %q onto %q", br, dbranch)
		if _, err := git("rebase", dbranch, br); err != nil {
			log.Fatalf("Rebase failed: %v", err)
		}
		if !*doForcePush {
			continue
		}
		log.Printf("Force pushing %q to %s", br, *useRemote)
		if _, err := git("push", "-f", *useRemote, br); err != nil {
			log.Fatalf("Force updating %q: %v", br, err)
		}
	}
}

func currentBranch() (string, error) { return git("branch", "--show-current") }

func branchesWithRemotes(matching, dbranch, useRemote string) ([]string, error) {
	localOut, err := git("branch", "--list", matching)
	if err != nil {
		return nil, err
	}
	local := strings.Split(strings.TrimSpace(localOut), "\n")
	for i, s := range local {
		local[i] = strings.TrimPrefix(strings.TrimSpace(s), "* ")
	}

	remoteOut, err := git("branch", "--list", "-r", useRemote+"/"+matching)
	if err != nil {
		return nil, err
	}
	remote := strings.Split(strings.TrimSpace(remoteOut), "\n")
	for i, s := range remote {
		remote[i] = strings.TrimPrefix(strings.TrimSpace(s), useRemote+"/")
	}

	var out []string
	for _, lb := range local {
		if lb == dbranch {
			continue // don't consider the default branch regardless
		}
		for _, rb := range remote {
			if rb == lb {
				out = append(out, lb)
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

func defaultBranch(useRemote string) (string, error) {
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

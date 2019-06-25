// Program repodeps scans the contents of a collection of GitHub repositories
// and reports the names and dependencies of any Go packages defined inside.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/build"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	doSourceHash = flag.Bool("sourcehash", false, "Record the names and digests of source files")
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatalf("Usage: %s <repo-dir> ...", filepath.Base(os.Args[0]))
	}
	ctx := context.Background()
	for _, dir := range flag.Args() {
		path, err := filepath.Abs(dir)
		if err != nil {
			log.Fatalf("Resolving path: %v", err)
		}
		repo, err := visitDirectory(ctx, path)
		if err != nil {
			log.Printf("Skipped %q:\n  %v", dir, err)
			continue
		}
		if err = json.NewEncoder(os.Stdout).Encode(repo); err != nil {
			log.Fatalf("Writing JSON: %v", err)
		}
	}
}

func visitDirectory(ctx context.Context, dir string) (*Repo, error) {
	// Find the URLs for the remotes defined for this repository.
	remotes, err := gitRemotes(ctx, dir)
	if err != nil {
		return nil, fmt.Errorf("listing remotes: %v", err)
	} else if len(remotes) == 0 {
		return nil, errors.New("no remotes defined")
	}

	repo := &Repo{Remotes: remotes}

	// Find the import paths of the packages defined by this repository, and the
	// import paths of their dependencies. This is basically "go list".
	err = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if !fi.IsDir() {
			return nil // nothing to do here
		} else if filepath.Base(path) == ".git" {
			return filepath.SkipDir
		}
		pkg, err := build.Default.ImportDir(path, 0)
		if err != nil {
			log.Printf("[skipping] %v", err)
			return nil
		}
		rec := &Package{
			Name:       pkg.Name,
			ImportPath: pkg.ImportPath,
			Imports:    pkg.Imports,
		}
		if *doSourceHash {
			for _, name := range pkg.GoFiles {
				path := filepath.Join(path, name)
				hash, err := hashFile(path)
				if err != nil {
					log.Printf("Hashing %q failed: %v", path, err)
				}
				rec.Source = append(rec.Source, &File{
					Name:   name,
					Digest: hash,
				})
			}
		}
		repo.Packages = append(repo.Packages, rec)
		return nil
	})
	return repo, err
}

// Repo records the Go package structure of a repository.
type Repo struct {
	Remotes  []*Remote  `json:"remotes,omitempty"`
	Packages []*Package `json:"packages,omitempty"`
}

// Package records the name and dependencies of a package for JSON encoding.
type Package struct {
	Name       string   `json:"name"`
	ImportPath string   `json:"importPath"`
	Imports    []string `json:"imports,omitempty"`
	Source     []*File  `json:"source,omitempty"`
}

// A Remote records the name and URL of a Git remote.
type Remote struct {
	Name string
	URL  string
}

// A File records the name and content digest of a file.
type File struct {
	Name   string `json:"name"`
	Digest []byte `json:"sha256"`
}

func gitRemotes(ctx context.Context, dir string) ([]*Remote, error) {
	cmd := exec.CommandContext(ctx, "git", "remote")
	cmd.Dir = dir
	bits, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing remotes: %v", err)
	}

	names := strings.Split(strings.TrimSpace(string(bits)), "\n")
	var rs []*Remote
	for _, name := range names {
		cmd := exec.CommandContext(ctx, "git", "remote", "get-url", name)
		cmd.Dir = dir
		bits, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("getting remote URL for %q: %v", name, err)
		}
		rs = append(rs, &Remote{Name: name, URL: parseRemote(bits)})
	}
	return rs, nil
}

func parseRemote(bits []byte) string {
	url := strings.TrimSpace(string(bits))
	if trim := strings.TrimPrefix(url, "git@"); trim != url {
		parts := strings.SplitN(trim, ":", 2)
		url = "https://" + parts[0] + "/"
		if len(parts) == 2 {
			url += parts[1]
		}
	}
	return strings.TrimSuffix(url, ".git")
}

func hashFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	_, err = io.Copy(h, f)
	return h.Sum(nil), err
}

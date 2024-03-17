package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/creachadair/mds/mapset"
	"github.com/creachadair/mds/mstr"
	git "github.com/go-git/go-git/v5"
	"github.com/google/go-github/v60/github"
)

// Dir manages a directory containing cloned gist repositories.
type Dir string

// Set implements part of flag.Value.
func (d *Dir) Set(s string) error { *d = Dir(s); return nil }

// String implements part of flag.Value.
func (d Dir) String() string { return string(d) }

// Init creates the directory managed by d if it does not exist.
func (d Dir) Init() error { return os.MkdirAll(string(d), 0755) }

// List returns a set of the gist digests currently stored in d.  The result is
// empty without error if there are none.
func (d Dir) List() (mapset.Set[string], error) {
	es, err := os.ReadDir(string(d))
	if err != nil {
		return nil, err
	}
	have := mapset.NewSize[string](len(es))
	for _, e := range es {
		if !e.IsDir() || !isHex(e.Name()) {
			continue
		}
		have.Add(e.Name())
	}
	return have, nil
}

// Path returns the path of the specified gist hash.  This does not imply the
// gist exists locally.
func (d Dir) Path(hash string) string { return filepath.Join(string(d), hash) }

// Create creates a new directory in d for the specified gist hash.
// It reports os.ErrExist if the hash already exists in d.
func (d Dir) Create(hash string) (string, error) {
	path := d.Path(hash)
	if _, err := os.Stat(path); err == nil {
		return path, os.ErrExist
	}
	return path, os.MkdirAll(path, 0700)
}

// Remove recursively removes hash from d. It returns nil if the directory for
// hash does not exist.
func (d Dir) Remove(hash string) error { return os.RemoveAll(d.Path(hash)) }

func isHex(s string) bool {
	for _, c := range s {
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' {
			continue
		}
		return false
	}
	return true
}

func listGists(ctx context.Context, token string) ([]*github.Gist, error) {
	cli := github.NewClient(nil).WithAuthToken(flags.Token)
	var all []*github.Gist
	for page := 1; ; page++ {
		gs, rsp, err := cli.Gists.List(ctx, "" /* auth user */, &github.GistListOptions{
			ListOptions: github.ListOptions{Page: page, PerPage: 100},
		})
		if err != nil {
			return nil, fmt.Errorf("list gists: %w", err)
		}
		all = append(all, gs...)
		if rsp.NextPage == 0 {
			break
		}
	}
	return all, nil
}

func vid(id string) string { return mstr.Trunc(id, 12) }

func cloneGist(ctx context.Context, id, pullURL string, d Dir) error {
	path, err := d.Create(id)
	if err != nil {
		return fmt.Errorf("create workdir: %w", err)
	}
	if _, err := git.PlainCloneContext(ctx, path, false /* not bare */, &git.CloneOptions{
		URL: pullURL,
	}); err != nil {
		return fmt.Errorf("clone %q: %w", vid(id), err)
	}
	return nil
}

func fetchGist(ctx context.Context, id string, d Dir) (bool, error) {
	repo, err := git.PlainOpen(d.Path(id))
	if err != nil {
		return false, fmt.Errorf("open %q: %w", vid(id), err)
	}
	if err := repo.FetchContext(ctx, &git.FetchOptions{}); errors.Is(err, git.NoErrAlreadyUpToDate) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("update %q: %w", vid(id), err)
	}
	return true, nil
}

func vlog(msg string, args ...any) {
	if flags.Verbose {
		log.Printf(msg, args...)
	}
}

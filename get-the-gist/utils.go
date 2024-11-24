package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/creachadair/mds/mapset"
	"github.com/creachadair/mds/mstr"
	git "github.com/go-git/go-git/v5"
	"github.com/google/go-github/v66/github"
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
	cli := github.NewClient(nil).WithAuthToken(token)
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
	repo, err := git.PlainCloneContext(ctx, path, false /* not bare */, &git.CloneOptions{
		URL: pullURL,
	})
	if err != nil {
		return fmt.Errorf("clone %q: %w", vid(id), err)
	}

	// Pull the HTTP fetch URL out of the first remote, and rewrite it to SSH
	// format so a human operator can pull and push from the work tree.
	url, rem, err := remoteURL(id, repo)
	if err != nil {
		return fmt.Errorf("remote URL for %q: %w", vid(id), err)
	}
	config := rem.Config()
	if want := httpToGitURL(url); config.URLs[0] != want {
		config.URLs = []string{want}
		if err := repo.DeleteRemote(config.Name); err != nil {
			return fmt.Errorf("delete remote %q: %w", config.Name, err)
		}
		if _, err := repo.CreateRemote(config); err != nil {
			return fmt.Errorf("update remote %q: %w", config.Name, err)
		}
	}
	return nil
}

func pullGist(ctx context.Context, id string, d Dir) (bool, error) {
	repo, err := git.PlainOpen(d.Path(id))
	if err != nil {
		return false, fmt.Errorf("open %q: %w", vid(id), err)
	}
	work, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("get %q worktree: %w", vid(id), err)
	}

	// Fetch using HTTP so that we don't need to tangle with SSH.
	// Gists are all readable without auth anyway, even if secret.
	url, _, err := remoteURL(id, repo)
	if err != nil {
		return false, fmt.Errorf("remote URL for %q: %w", vid(id), err)
	}
	if err := work.PullContext(ctx, &git.PullOptions{
		RemoteURL: gitURLToHTTP(url),
	}); errors.Is(err, git.NoErrAlreadyUpToDate) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("update %q: %w", vid(id), err)
	}
	return true, nil
}

func remoteURL(id string, repo *git.Repository) (string, *git.Remote, error) {
	// Find the remote URL from the first remote. Usually there will only be
	// one, that is to say the origin from the clone.
	rems, err := repo.Remotes()
	if err != nil {
		return "", nil, fmt.Errorf("list remotes %q: %w", vid(id), err)
	}
	if len(rems) == 0 {
		return "", nil, fmt.Errorf("no remotes for %q", vid(id))
	}
	return rems[0].Config().URLs[0], rems[0], nil
}

func httpToGitURL(httpURL string) string {
	u, err := url.Parse(httpURL)
	if err != nil {
		return httpURL
	}
	return fmt.Sprintf("git@%s:%s", u.Host, strings.TrimPrefix(u.Path, "/"))
}

func gitURLToHTTP(gitURL string) string {
	tail, ok := strings.CutPrefix(gitURL, "git@")
	if !ok {
		return gitURL
	}
	host, path, ok := strings.Cut(tail, ":")
	if !ok {
		return gitURL
	}
	return fmt.Sprintf("https://%s/%s", host, path)
}

func vlog(msg string, args ...any) {
	if flags.Verbose {
		log.Printf(msg, args...)
	}
}

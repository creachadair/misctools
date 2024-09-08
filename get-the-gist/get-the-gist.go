// Program get-the-gist synchronizes a copy of the user's GitHub gists to
// a directory on the local disk.
package main

import (
	"cmp"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/creachadair/command"
	"github.com/creachadair/flax"
	"github.com/creachadair/mds/mapset"
	"github.com/creachadair/taskgroup"
)

var flags struct {
	Token   string `flag:"token,GitHub API token (required)"`
	Dir     Dir    `flag:"dir,Output directory (required)"`
	Cleanup bool   `flag:"cleanup,Remove gists not found on GitHub"`
	Verbose bool   `flag:"v,Enable verbose logging"`
}

func main() {
	root := &command.C{
		Name: command.ProgramName(),
		Help: `Fetch and/or update a local copy of your GitHub gists.

Either pass a GitHub auth token as --token, or populate the GISTBOT_TOKEN
environment variable with same. The token must have permission to read gists.

The output directory --dir is created if it does not exist.`,

		SetFlags: command.Flags(flax.MustBind, &flags),
		Run:      command.Adapt(runMain),

		Commands: []*command.C{
			command.HelpCommand(nil),
			command.VersionCommand(),
		},
	}
	command.RunOrFail(root.NewEnv(nil), os.Args[1:])
}

func runMain(env *command.Env) error {
	token := cmp.Or(flags.Token, os.Getenv("GISTBOT_TOKEN"))
	if token == "" {
		return env.Usagef("you must provide a --token or set GISTBOT_TOKEN")
	}
	if flags.Dir == "" {
		return env.Usagef("you must provide an output --dir")
	} else if err := flags.Dir.Init(); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	have, err := flags.Dir.List()
	if err != nil {
		return fmt.Errorf("list output: %w", err)
	}
	vlog("Found %d gists in output directory", len(have))

	exist, err := listGists(env.Context(), token)
	if err != nil {
		return fmt.Errorf("list remote: %w", err)
	}
	vlog("Found %d gists on GitHub", len(exist))

	g, start := taskgroup.New(taskgroup.Listen(env.Cancel)).Limit(5)

	type update struct {
		id string
		ok bool
	}
	var checked mapset.Set[string]
	var updates, fetches int
	coll := taskgroup.Collect(func(u update) {
		checked.Add(u.id)
		if u.ok {
			updates++
		}
	})

	for _, e := range exist {
		if have.Has(e.GetID()) {
			start(coll.Call(func() (update, error) {
				start := time.Now()
				ok, err := pullGist(env.Context(), e.GetID(), flags.Dir)
				if err != nil {
					return update{}, err
				} else if ok {
					log.Printf("Updated gist %q (%v elapsed)", vid(e.GetID()), time.Since(start).Round(time.Millisecond))
				} else {
					vlog("Gist %q is up-to-date", vid(e.GetID()))
				}
				return update{e.GetID(), ok}, nil
			}))
		} else {
			fetches++
			start(func() error {
				start := time.Now()
				if err := cloneGist(env.Context(), e.GetID(), e.GetGitPullURL(), flags.Dir); err != nil {
					return err
				}
				log.Printf("Fetched new gist %q (%v elapsed)",
					vid(e.GetID()), time.Since(start).Round(time.Millisecond))
				return nil
			})
		}
	}
	if err := g.Wait(); err != nil {
		return err
	}

	have.RemoveAll(checked)
	log.Printf("Found %d gists, fetched %d, checked %d, updated %d, gone %d",
		len(exist), fetches, len(checked), updates, len(have))

	if len(have) == 0 {
		return nil
	} else if !flags.Cleanup {
		log.Printf("NOTE: Found %d gists locally that are not on GitHub", len(have))
		log.Printf("      Run with --cleanup to remove them")
		vlog("IDs: %v", have.Slice())
		return nil
	}

	for id := range have {
		if err := flags.Dir.Remove(id); err != nil {
			return fmt.Errorf("remove gist: %w", err)
		}
		log.Printf("Removed deleted gist %q", vid(id))
	}
	return nil
}

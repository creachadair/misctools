package gitblep_test

import (
	"flag"
	"testing"

	"github.com/creachadair/misctools/gitblep/gitblep"
)

var (
	gitDir  = flag.String("git", "", "Path of git directory")
	objHash = flag.String("hash", "", "Object hash to read")
)

func TestParseBlob(t *testing.T) {
	if *gitDir == "" || *objHash == "" {
		t.Skip("No inputs are defined")
	}

	r := gitblep.Repo{Dir: *gitDir}

	p, err := r.Pack(*objHash)
	if err != nil {
		t.Fatalf("Object: %v", err)
	}
	t.Logf("Pack: %+v", p)
}

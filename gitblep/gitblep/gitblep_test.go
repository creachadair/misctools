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

	obj, err := r.Object(*objHash)
	if err != nil {
		t.Fatalf("Object: %v", err)
	}
	t.Logf("Loaded %s %q (%d bytes)", obj.Type, obj.Hash, len(obj.Data))

	var c gitblep.Commit
	if err := c.UnmarshalBinary(obj.Data); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	t.Logf("Commit: %v", c)

	tobj, err := r.Object(c.Tree)
	if err != nil {
		t.Fatalf("Object: %v", err)
	}

	var tree gitblep.Tree
	if err := tree.UnmarshalBinary(tobj.Data); err != nil {
		t.Fatalf("Tree: %v", err)
	}
	for i, v := range tree {
		t.Logf("%d: %v", i+1, v)
	}
}

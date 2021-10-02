package group_test

import (
	"testing"

	"github.com/creachadair/misctools/group"
)

func TestLookup(t *testing.T) {
	const name = "nogroup"

	g1, err := group.Lookup(name)
	if err != nil {
		t.Fatalf("Lookup(%q): unexpected error: %v", name, err)
	}
	t.Logf("Lookup(%q): %#v", name, g1)
	if g1.Name != name {
		t.Errorf("Lookup(%q): got name %q, want %q", name, g1.Name, name)
	}

	g2, err := group.LookupID(g1.ID)
	if err != nil {
		t.Fatalf("LookupID(%d): unexpected error: %v", g1.ID, err)
	}
	t.Logf("LookupID(%d): %#v", g1.ID, g2)
	if g2.ID != g1.ID {
		t.Errorf("LookupID(%d): got id %d, want %d", g1.ID, g2.ID, g1.ID)
	}

	const noSuchUser = "no-such-user-nohow"
	g3, err := group.Lookup(noSuchUser)
	if err == nil {
		t.Errorf("Lookup(%q): got %v, want error", noSuchUser, g3)
	} else {
		t.Logf("Lookup(%q): got expected error: %v", noSuchUser, err)
	}
}

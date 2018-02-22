package group

import "testing"

func TestLookup(t *testing.T) {
	const name = "nobody"

	g1, err := Lookup(name)
	if err != nil {
		t.Fatalf("Lookup(%q): unexpected error: %v", name, err)
	}
	t.Logf("Lookup(%q): %#v", name, g1)
	if g1.Name != name {
		t.Errorf("Lookup(%q): got name %q, want %q", name, g1.Name, name)
	}

	g2, err := LookupID(g1.ID)
	if err != nil {
		t.Fatalf("LookupID(%d): unexpected error: %v", g1.ID, err)
	}
	t.Logf("LookupID(%d): %#v", g1.ID, g2)
	if g2.ID != g1.ID {
		t.Errorf("LookupID(%d): got id %d, want %d", g1.ID, g2.ID, g1.ID)
	}
}

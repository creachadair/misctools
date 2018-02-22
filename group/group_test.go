package group

import "testing"

func TestLookup(t *testing.T) {
	const name = "nobody"

	g1, err := ByName(name)
	if err != nil {
		t.Fatalf("ByName(%q): unexpected error: %v", name, err)
	}
	t.Logf("ByName(%q): %#v", name, g1)
	if g1.Name != name {
		t.Errorf("ByName(%q): got name %q, want %q", name, g1.Name, name)
	}

	g2, err := ByGID(g1.ID)
	if err != nil {
		t.Fatalf("ByGID(%d): unexpected error: %v", g1.ID, err)
	}
	t.Logf("ByGID(%d): %#v", g1.ID, g2)
	if g2.ID != g1.ID {
		t.Errorf("ByGID(%d): got id %d, want %d", g1.ID, g2.ID, g1.ID)
	}
}

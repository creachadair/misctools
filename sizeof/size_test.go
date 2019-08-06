package sizeof_test

import (
	"testing"

	"github.com/creachadair/misctools/sizeof"
)

func TestCycles(t *testing.T) {
	type V struct {
		Z int
		E *V
	}

	v := &V{Z: 25}
	want := sizeof.DeepSize(v)
	v.E = v // induce a cycle
	got := sizeof.DeepSize(v)
	if got != want {
		t.Errorf("Cyclic size: got %d, want %d", got, want)
	}
}

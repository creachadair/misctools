package paths

import (
	"log"
	"os"
	"testing"
)

func TestRealpath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Getcwd: unexpected error: %v", err)
	}
	p, err := Realpath(".")
	if err != nil {
		t.Fatalf("Realpath(.): unexpected error: %v", err)
	}
	t.Logf("Realpath(.): %q", p)
	if p != cwd {
		t.Errorf("Realpath(.): got %q, want %q", p, cwd)
	}
}

func TestRealpathError(t *testing.T) {
	p, err := Realpath("bogus")
	if err != nil {
		t.Logf("Realpath(bogus): got expected error: %v", err)
	} else {
		t.Errorf("Realpath(bogus): got %q, wanted error", p)
	}
}

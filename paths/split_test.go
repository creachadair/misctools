package paths

import "testing"

func TestSplitExt(t *testing.T) {
	tests := []struct {
		input     string
		base, ext string
	}{
		{"", "", ""},
		{"noext", "noext", ""},
		{".nobase", "", ".nobase"},
		{"last.ext.only", "last.ext", ".only"},
		{"leave/path.alone", "leave/path", ".alone"},
	}
	for _, test := range tests {
		base, ext := SplitExt(test.input)
		if base != test.base {
			t.Errorf("SplitExt %q: got base %q, want %q", test.input, base, test.base)
		}
		if ext != test.ext {
			t.Errorf("SplitExt %q: got ext %q, want %q", test.input, ext, test.ext)
		}
	}
}

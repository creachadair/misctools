package paths

import (
	"path/filepath"
	"strings"
)

// SplitExt splits the file extension from name, as defined by filepath.Ext.
//	It returns the name without extension, along with the extension.
func SplitExt(name string) (base, ext string) {
	ext = filepath.Ext(name)
	base = strings.TrimSuffix(name, ext)
	return
}

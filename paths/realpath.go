package paths

// #include <stdlib.h>
import "C"
import (
	"unsafe"
)

// Realpath resolves all symbolic links, extra separators, and relative
// references ("." and "..")  in path.
func Realpath(path string) (string, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	cexp, err := C.realpath(cpath, nil)
	if cexp == nil {
		return "", err
	}
	defer C.free(unsafe.Pointer(cexp))
	return C.GoString(cexp), nil
}

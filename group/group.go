// Package group provides an interface to the getgrnam(3) and getgrgid(3)
// functions to read group membership from the /etc/groups database, NIS, LDAP,
// etc. This package will only work on os/arch combinations where those
// functions are defined.
package group

// #include <grp.h>
// #include <sys/types.h>
import "C"
import (
	"errors"
	"unsafe"
)

// A Group represents an authorization group.
type Group struct {
	Name     string
	Password string
	ID       int
	Members  []string

	// N.B. POSIX is not specific about whether gid_t is signed.  This package
	// defines it as signed to handle both possibilities.
}

// ErrNotFound is returned when a requested group could not be found.
var ErrNotFound = errors.New("no matching group")

// Lookup loads the named group via getgrnam(3).
// If the group could not be found, ErrNotFound is reported.
func Lookup(name string) (*Group, error) {
	grp, err := C.getgrnam(C.CString(name))
	if err != nil {
		return nil, err
	}
	return unpack(grp)
}

// LookupID loads the specified group via getgrgid(3).
// If the group could not be found, ErrNotFound is reported.
func LookupID(gid int) (*Group, error) {
	grp, err := C.getgrgid(C.gid_t(gid))
	if err != nil {
		return nil, err
	}
	return unpack(grp)
}

func unpack(grp *C.struct_group) (*Group, error) {
	// The lookup functions do not report an error for a missing group, they
	// just return a NULL.
	if grp == nil {
		return nil, ErrNotFound
	}

	// The members list is a NULL-terminated array of C strings.  We need
	// unsafe here to handle the pointer arithmetic.
	var mem []string
	p := grp.gr_mem
	q := unsafe.Pointer(p)
	for i := uintptr(0); ; i++ {
		s := (**C.char)(unsafe.Pointer(uintptr(q) + i*unsafe.Sizeof(q)))
		if *s == nil {
			break
		}
		mem = append(mem, C.GoString(*s))
	}
	return &Group{
		Name:     C.GoString(grp.gr_name),
		Password: C.GoString(grp.gr_passwd),
		ID:       int(C.int(grp.gr_gid)),
		Members:  mem,
	}, nil
}

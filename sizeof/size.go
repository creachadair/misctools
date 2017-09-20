// Package sizeof provides utility functions for recursively computing the size
// of Go objects in memory, using the reflect package.
package sizeof

import (
	"math"
	"reflect"
)

// DeepSize reports the size of v in bytes, as reflect.Size, but also including
// all recursive substructures of v via pointers and slices. If v contains any
// cycles (even indirect ones), this function may loop.
//
// Note that some values, notably maps, may still be undercounted, as there are
// some pieces of the value that are not visible to reflect.Size.
func DeepSize(v interface{}) int64 {
	return int64(valueSize(reflect.ValueOf(v)))
}

func valueSize(v reflect.Value) uintptr {
	base := v.Type().Size()
	switch v.Kind() {
	case reflect.Ptr:
		if !v.IsNil() {
			return base + valueSize(v.Elem())
		}

	case reflect.Slice:
		n := v.Len()
		for i := 0; i < n; i++ {
			base += valueSize(v.Index(i))
		}

		// Account for the parts of the array not covered by this slice.  Since
		// we can't get the values directly, assume they're zeroes. That may be
		// incorrect, in which case we may underestimate.
		if cap := v.Cap(); cap > n {
			base += v.Type().Size() * uintptr(cap-n)
		}

	case reflect.Map:
		// A map m has len(m) / 6.5 buckets, rounded up to a power of two, and
		// a minimum of one bucket. Each bucket is 16 bytes + 8*(keysize + valsize).
		nb := uintptr(math.Pow(2, math.Ceil(math.Log(float64(v.Len())/6.5)/math.Log(2))))
		base = 16 * nb
		for _, key := range v.MapKeys() {
			base += valueSize(key)
			base += valueSize(v.MapIndex(key))
		}

	case reflect.Struct:
		// Chase pointer and slice fields and add the size of their members.
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			switch f.Kind() {
			case reflect.Ptr:
				if !f.IsNil() {
					base += valueSize(f.Elem())
				}
			case reflect.Slice:
				base += valueSize(f)
			}
		}

	case reflect.String:
		return base + uintptr(v.Len())

	}
	return base
}

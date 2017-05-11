// Package sizeof provides utility functions for recursively computing the size
// of Go objects in memory, using the reflect package.
package sizeof

import "reflect"

// DeepSize reports the size of v in bytes, including all recursive
// substructures of v. If v contains any cycles, this function may loop.
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

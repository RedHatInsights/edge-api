// Package ptr provides value to pointer and pointer to value functions.
//
// In most cases, simply use ptr.To or ptr.From functions. There are some
// extra functions for types which would normally need type conversions
// from constants, such as ptr.ToInt64.
package ptr

// To returns a pointer to the given value.
func To[T any](value T) *T {
	return &value
}

// From returns the value from a given pointer. If ref is nil, a zero
// value of type T will be returned.
func From[T any](ref *T) (value T) {
	if ref != nil {
		value = *ref
	}
	return
}

// FromOrEmpty returns the value or empty value in case the value is nil.
func FromOrEmpty[T any](ref *T) (value T) {
	if ref != nil {
		value = *ref
	} else {
		value = *new(T)
	}
	return
}

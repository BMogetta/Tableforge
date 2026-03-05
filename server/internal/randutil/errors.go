package randutil

import "errors"

var (
	// ErrInvalidBound is returned when Intn is called with n <= 0.
	ErrInvalidBound = errors.New("randutil: n must be greater than 0")

	// ErrEmptySlice is returned when Pick is called on an empty slice.
	ErrEmptySlice = errors.New("randutil: slice must not be empty")
)

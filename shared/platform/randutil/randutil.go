// Package randutil provides cryptographically secure random utilities.
// All randomness in Recess (first mover selection, card shuffling, etc.)
// should go through this package instead of using crypto/rand inline.
package randutil

import (
	"crypto/rand"
	"math/big"
)

// Intn returns a cryptographically secure random integer in [0, n).
// Returns an error if n <= 0 or if the underlying random source fails.
func Intn(n int) (int, error) {
	if n <= 0 {
		return 0, ErrInvalidBound
	}
	val, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(val.Int64()), nil
}

// Shuffle returns a new slice with the elements of s in a random order.
// The original slice is not modified.
func Shuffle[T any](s []T) ([]T, error) {
	out := make([]T, len(s))
	copy(out, s)
	// Fisher-Yates shuffle
	for i := len(out) - 1; i > 0; i-- {
		j, err := Intn(i + 1)
		if err != nil {
			return nil, err
		}
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// ShuffleWith returns a new slice with the elements of s in a random order
// using the provided RNG function. rng(n) must return a value in [0, n).
// Use this for deterministic, reproducible shuffles (e.g. seeded PRNG).
func ShuffleWith[T any](s []T, rng func(n int) int) []T {
	out := make([]T, len(s))
	copy(out, s)
	for i := len(out) - 1; i > 0; i-- {
		j := rng(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// Pick returns a single random element from s.
// Returns an error if s is empty.
func Pick[T any](s []T) (T, error) {
	var zero T
	if len(s) == 0 {
		return zero, ErrEmptySlice
	}
	i, err := Intn(len(s))
	if err != nil {
		return zero, err
	}
	return s[i], nil
}

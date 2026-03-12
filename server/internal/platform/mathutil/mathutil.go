// Package mathutil provides shared mathematical helpers used by the rating
// and simulation packages.
package mathutil

import "math"

// Sigmoid returns the logistic expected score of a player with rating
// difference diff against an opponent, using the given scale factor.
//
//	E = 1 / (1 + 10^(-diff / scale))
//
// In standard Elo, scale = 400. A positive diff means the player is
// favoured; a negative diff means the opponent is favoured.
func Sigmoid(diff, scale float64) float64 {
	return 1.0 / (1.0 + math.Pow(10, -diff/scale))
}

// Clamp returns v clamped to the closed interval [lo, hi].
func Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Mean returns the arithmetic mean of a non-empty slice.
// Returns 0 for an empty slice.
func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// Variance returns the population variance of a non-empty slice.
// Returns 0 for slices of length < 2.
func Variance(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := Mean(xs)
	sum := 0.0
	for _, x := range xs {
		d := x - m
		sum += d * d
	}
	return sum / float64(len(xs))
}

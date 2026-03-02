package ratelimit

import "time"

// Clock abstracts time access to make the limiter deterministic in tests.
type Clock interface {
	Now() time.Time
}

// RealClock implements Clock using the system time.
type RealClock struct{}

// Now returns the current system time.
func (RealClock) Now() time.Time {
	return time.Now()
}

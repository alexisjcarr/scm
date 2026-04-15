package clock

import "time"

// Clock allows tests to control time-sensitive behavior.
type Clock interface {
	Now() time.Time
}

// RealClock uses the system clock.
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now().UTC()
}

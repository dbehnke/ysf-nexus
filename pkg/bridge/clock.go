package bridge

import "time"

// Clock is a small abstraction over time so we can inject deterministic times in tests
type Clock interface {
	Now() time.Time
}

// RealClock uses the real time.Now
type RealClock struct{}

func (r *RealClock) Now() time.Time { return time.Now() }

// FakeClock returns a fixed time; tests can set NowTime to the desired value
type FakeClock struct {
	NowTime time.Time
}

func (f *FakeClock) Now() time.Time { return f.NowTime }

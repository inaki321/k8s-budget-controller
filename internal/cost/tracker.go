package cost

import "time"

type Tracker struct {
	window      time.Duration
	cpuPrice    float64 // USD per core-hour
	memPrice    float64 // USD per GiB-hour
	spend       float64
	windowStart time.Time
	last        time.Time
}

func NewTracker(now time.Time, window time.Duration, cpuPrice, memPrice float64) *Tracker {
	return &Tracker{window: window, cpuPrice: cpuPrice, memPrice: memPrice, windowStart: now, last: now}
}

// Observe records the current usage and returns the window's spend so far.
// reset is true when a new budget window just started.
func (t *Tracker) Observe(now time.Time, cpuCores, memGiB float64) (spend float64, reset bool) {
	if now.Sub(t.windowStart) >= t.window {
		t.spend = 0
		t.windowStart = now
		t.last = now
		return 0, true
	}
	hours := now.Sub(t.last).Hours()
	t.spend += (cpuCores*t.cpuPrice + memGiB*t.memPrice) * hours
	t.last = now
	return t.spend, false
}

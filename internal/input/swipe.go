package input

import "fmt"

// Swipe moves one finger between portrait screenshot positions.
type Swipe struct {
	From       Position `json:"from"`
	To         Position `json:"to"`
	DurationMS int      `json:"duration_ms"`
}

func (s Swipe) Validate() error {
	if !validPosition(s.From.X, s.From.Y) || !validPosition(s.To.X, s.To.Y) {
		return fmt.Errorf("swipe endpoints must lie inside %dx%d portrait screenshot", Width, Height)
	}
	if s.From == s.To {
		return fmt.Errorf("swipe endpoints must differ; use tap for stationary contact")
	}
	if s.DurationMS < 1 || s.DurationMS > 5000 {
		return fmt.Errorf("swipe duration must be 1ms..5s")
	}
	return nil
}

func (s Swipe) samples() []Point {
	return (Stroke{sampleIntervalMS: 8, Points: []Point{
		{X: s.From.X, Y: s.From.Y},
		{X: s.To.X, Y: s.To.Y, TimeMS: s.DurationMS},
	}}).Samples()
}

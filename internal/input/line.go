package input

import (
	"fmt"
	"math"
)

// Position is expressed in portrait screenshot pixels.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Line uses the tablet's hold-to-snap behavior after drawing a straight path.
// Snapping depends on the selected tool and xochitl; it cannot be acknowledged
// through evdev, so success means the gesture was sent, not that snapping occurred.
type Line struct {
	From       Position `json:"from"`
	To         Position `json:"to"`
	DurationMS int      `json:"duration_ms"`
	HoldMS     int      `json:"hold_ms"`
	Pressure   float64  `json:"pressure"`
}

func (l Line) Validate() error {
	if !validPosition(l.From.X, l.From.Y) || !validPosition(l.To.X, l.To.Y) {
		return fmt.Errorf("line endpoints must lie inside %dx%d portrait screenshot", Width, Height)
	}
	if l.From == l.To {
		return fmt.Errorf("line endpoints must differ")
	}
	if l.DurationMS < 1 || l.DurationMS > 4000 {
		return fmt.Errorf("line duration must be 1ms..4s")
	}
	if l.HoldMS < 800 || l.HoldMS > 5000 {
		return fmt.Errorf("line snap hold must be 800ms..5s")
	}
	if math.IsNaN(l.Pressure) || l.Pressure <= 0 || l.Pressure > 1 {
		return fmt.Errorf("line pressure must be greater than 0 and at most 1")
	}
	return nil
}

func (l Line) Stroke() (Stroke, error) {
	if err := l.Validate(); err != nil {
		return Stroke{}, err
	}
	p := l.Pressure
	s := Stroke{sampleIntervalMS: 8, Points: []Point{{X: l.From.X, Y: l.From.Y, Pressure: &p}, {X: l.To.X, Y: l.To.Y, TimeMS: l.DurationMS, Pressure: &p}}}
	// Unchanged coordinates are filtered by evdev. Tiny endpoint motion keeps
	// reports flowing while xochitl recognizes the hold; none is added in transit.
	for t := 8; t < l.HoldMS; t += 8 {
		dx := 0.2
		if t%16 == 0 {
			dx = -dx
		}
		x := max(0, math.Min(Width-1, l.To.X+dx))
		s.Points = append(s.Points, Point{X: x, Y: l.To.Y, TimeMS: l.DurationMS + t, Pressure: &p})
	}
	s.Points = append(s.Points, Point{X: l.To.X, Y: l.To.Y, TimeMS: l.DurationMS + l.HoldMS, Pressure: &p})
	return s, s.Validate()
}

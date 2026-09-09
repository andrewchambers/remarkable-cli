package input

import (
	"fmt"
	"math"
)

// NewStroke assigns time in proportion to distance traveled, using the helper's
// millisecond timeline. Repeated adjacent positions add no movement and are ignored.
func NewStroke(positions []Position, durationMS int, pressure float64) (Stroke, error) {
	if len(positions) < 2 || len(positions) > 10000 {
		return Stroke{}, fmt.Errorf("stroke requires 2..10000 points")
	}
	if durationMS < 1 || durationMS > 10000 {
		return Stroke{}, fmt.Errorf("stroke duration must be 1ms..10s")
	}
	if math.IsNaN(pressure) || pressure <= 0 || pressure > 1 {
		return Stroke{}, fmt.Errorf("stroke pressure must be greater than 0 and at most 1")
	}
	var path []Position
	var distances []float64
	total := 0.0
	for i, p := range positions {
		if !validPosition(p.X, p.Y) {
			return Stroke{}, fmt.Errorf("point %d: coordinates outside portrait screenshot", i+1)
		}
		if len(path) > 0 {
			prev := path[len(path)-1]
			if p == prev {
				continue
			}
			total += math.Hypot(p.X-prev.X, p.Y-prev.Y)
		}
		path = append(path, p)
		distances = append(distances, total)
	}
	if len(path) < 2 {
		return Stroke{}, fmt.Errorf("stroke requires at least two distinct positions")
	}
	s := Stroke{Points: make([]Point, len(path))}
	for i, p := range path {
		t := int(math.Round(distances[i] / total * float64(durationMS)))
		if i == len(path)-1 {
			t = durationMS
		}
		if i > 0 && t <= s.Points[i-1].TimeMS {
			return Stroke{}, fmt.Errorf("points are too close together for this duration at millisecond precision; increase --duration or use fewer points")
		}
		s.Points[i] = Point{X: p.X, Y: p.Y, TimeMS: t, Pressure: &pressure}
	}
	return s, s.Validate()
}

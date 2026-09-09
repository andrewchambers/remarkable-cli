package input

import (
	"fmt"
	"math"
)

// Pinch moves two fingers horizontally and symmetrically about a fixed center.
// Distances are the full separation between fingers in screenshot pixels.
type Pinch struct {
	Center        Position `json:"center"`
	StartDistance float64  `json:"start_distance"`
	EndDistance   float64  `json:"end_distance"`
	DurationMS    int      `json:"duration_ms"`
}

func (p Pinch) Validate() error {
	if !validPosition(p.Center.X, p.Center.Y) {
		return fmt.Errorf("pinch center must lie inside %dx%d portrait screenshot", Width, Height)
	}
	for _, distance := range []float64{p.StartDistance, p.EndDistance} {
		if math.IsNaN(distance) || math.IsInf(distance, 0) || distance < 20 {
			return fmt.Errorf("pinch finger distances must be finite and at least 20 pixels")
		}
		left, right := p.positions(distance)
		if !validPosition(left.X, left.Y) || !validPosition(right.X, right.Y) {
			return fmt.Errorf("both pinch fingers must stay inside the portrait screenshot; reduce distances or move --center")
		}
	}
	if p.StartDistance == p.EndDistance {
		return fmt.Errorf("pinch start and end distances must differ")
	}
	if p.DurationMS < 1 || p.DurationMS > 5000 {
		return fmt.Errorf("pinch duration must be 1ms..5s")
	}
	return nil
}

func (p Pinch) positions(distance float64) (Position, Position) {
	return Position{X: p.Center.X - distance/2, Y: p.Center.Y},
		Position{X: p.Center.X + distance/2, Y: p.Center.Y}
}

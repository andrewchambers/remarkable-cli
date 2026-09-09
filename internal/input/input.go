// Package input defines gestures in portrait screenshot coordinates.
package input

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
)

const Width, Height = 1404, 1872

type Point struct {
	X        float64  `json:"x"`
	Y        float64  `json:"y"`
	TimeMS   int      `json:"t_ms"`
	Pressure *float64 `json:"pressure,omitempty"`
}
type Stroke struct {
	Points []Point `json:"points"`
	// Internal override for line's independently tested sampling cadence.
	// Ordinary stroke requests use the default 2 ms interval.
	sampleIntervalMS int
}
type Tap struct {
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	DurationMS int     `json:"duration_ms"`
}

func validPosition(x, y float64) bool {
	return !math.IsNaN(x) && !math.IsNaN(y) && x >= 0 && x < Width && y >= 0 && y < Height
}
func (t Tap) Validate() error {
	if !validPosition(t.X, t.Y) {
		return fmt.Errorf("coordinates must lie inside %dx%d portrait screenshot", Width, Height)
	}
	if t.DurationMS < 1 || t.DurationMS > 2000 {
		return fmt.Errorf("tap duration must be 1..2000 ms")
	}
	return nil
}
func (s Stroke) Validate() error {
	if len(s.Points) < 2 || len(s.Points) > 10000 {
		return fmt.Errorf("stroke requires 2..10000 points")
	}
	for i, p := range s.Points {
		if !validPosition(p.X, p.Y) {
			return fmt.Errorf("point %d: coordinates outside screen", i)
		}
		if p.TimeMS < 0 || p.TimeMS > 10000 || (i == 0 && p.TimeMS != 0) || (i > 0 && p.TimeMS <= s.Points[i-1].TimeMS) {
			return fmt.Errorf("point %d: times must start at zero, increase strictly, and end within 10000 ms", i)
		}
		if p.Pressure != nil && (math.IsNaN(*p.Pressure) || *p.Pressure <= 0 || *p.Pressure > 1) {
			return fmt.Errorf("point %d: pressure must be greater than 0 and at most 1", i)
		}
	}
	return nil
}
func Decode(r io.Reader, v any) error {
	data, err := io.ReadAll(io.LimitReader(r, 1024*1024+1))
	if err != nil {
		return err
	}
	if len(data) > 1024*1024 {
		return fmt.Errorf("gesture JSON exceeds 1 MiB")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected one JSON gesture")
	}
	return nil
}

// Samples interpolates positions and pressure every 2 ms by default, retaining
// all supplied points and their timestamps. Line gestures retain an 8 ms cadence.
func (s Stroke) Samples() []Point {
	interval := s.sampleIntervalMS
	if interval == 0 {
		interval = 2
	}
	result := []Point{s.Points[0]}
	for i := 1; i < len(s.Points); i++ {
		a, b := s.Points[i-1], s.Points[i]
		for t := a.TimeMS + interval; t < b.TimeMS; t += interval {
			f := float64(t-a.TimeMS) / float64(b.TimeMS-a.TimeMS)
			p := pressure(a) + (pressure(b)-pressure(a))*f
			result = append(result, Point{X: a.X + (b.X-a.X)*f, Y: a.Y + (b.Y-a.Y)*f, TimeMS: t, Pressure: &p})
		}
		result = append(result, b)
	}
	return result
}
func pressure(p Point) float64 {
	if p.Pressure == nil {
		return 0.5
	}
	return *p.Pressure
}

package input

import (
	"math"
	"testing"
)

func TestLineGesture(t *testing.T) {
	l := Line{From: Position{100, 200}, To: Position{500, 600}, DurationMS: 600, HoldMS: 2400, Pressure: 0.7}
	s, err := l.Stroke()
	if err != nil {
		t.Fatal(err)
	}
	samples := s.Samples()
	// Changing the general stroke default must not silently change line timing.
	if len(samples) != 376 {
		t.Fatalf("line sample count changed: %d", len(samples))
	}
	for i := 1; i < len(samples); i++ {
		if samples[i].TimeMS-samples[i-1].TimeMS != 8 {
			t.Fatal("line cadence changed")
		}
	}
	updates := 0
	for _, p := range samples {
		if p.TimeMS <= 600 {
			f := float64(p.TimeMS) / 600
			if math.Abs(p.X-(100+400*f)) > 1e-9 || math.Abs(p.Y-(200+400*f)) > 1e-9 {
				t.Fatal("line curves before endpoint")
			}
		} else {
			if math.Abs(p.X-500) > 0.201 || p.Y != 600 {
				t.Fatal("hold moved too far")
			}
			if p.X != 500 {
				updates++
			}
		}
		if pressure(p) != 0.7 {
			t.Fatal("pressure changed")
		}
	}
	last := samples[len(samples)-1]
	if last.TimeMS != 3000 || last.X != 500 || last.Y != 600 || updates == 0 {
		t.Fatal("bad hold or endpoint")
	}
}
func TestLineBoundaryHolds(t *testing.T) {
	for _, x := range []float64{0, Width - 1} {
		s, err := (Line{From: Position{10, 10}, To: Position{x, Height - 1}, DurationMS: 1, HoldMS: 801, Pressure: 0.5}).Stroke()
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range s.Points {
			if !validPosition(p.X, p.Y) {
				t.Fatal("hold escaped screen")
			}
		}
	}
}
func TestInvalidLines(t *testing.T) {
	base := Line{From: Position{10, 10}, To: Position{20, 20}, DurationMS: 600, HoldMS: 2400, Pressure: 0.5}
	for _, change := range []func(*Line){func(l *Line) { l.To = l.From }, func(l *Line) { l.To.X = math.Inf(1) }, func(l *Line) { l.DurationMS = 0 }, func(l *Line) { l.HoldMS = 799 }, func(l *Line) { l.Pressure = math.NaN() }, func(l *Line) { l.Pressure = 0 }, func(l *Line) { l.DurationMS = 4001 }, func(l *Line) { l.HoldMS = 5001 }} {
		l := base
		change(&l)
		if _, err := l.Stroke(); err == nil {
			t.Fatal("accepted invalid line")
		}
	}
}

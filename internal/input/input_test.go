package input

import (
	"math"
	"strings"
	"testing"
)

func pointer(v float64) *float64 { return &v }
func TestValidation(t *testing.T) {
	good := Stroke{Points: []Point{{X: 100, Y: 100}, {X: 200, Y: 200, TimeMS: 100}}}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, s := range []Stroke{
		{}, {Points: []Point{{}, {TimeMS: 0}}},
		{Points: []Point{{TimeMS: 1}, {TimeMS: 2}}},
		{Points: []Point{{}, {TimeMS: 10001}}},
		{Points: []Point{{X: Width}, {TimeMS: 1}}},
		{Points: []Point{{Y: -1}, {TimeMS: 1}}},
		{Points: []Point{{X: math.NaN()}, {TimeMS: 1}}},
		{Points: []Point{{Pressure: pointer(0)}, {TimeMS: 1}}},
		{Points: []Point{{Pressure: pointer(1.1)}, {TimeMS: 1}}},
	} {
		if err := s.Validate(); err == nil {
			t.Fatalf("accepted invalid stroke %+v", s)
		}
	}
	for _, tap := range []Tap{{DurationMS: 0}, {X: -1, DurationMS: 80}, {Y: Height, DurationMS: 80}, {DurationMS: 2001}} {
		if tap.Validate() == nil {
			t.Fatalf("accepted invalid tap %+v", tap)
		}
	}
	if err := (Tap{X: Width - 1, Y: Height - 1, DurationMS: 80}).Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestInterpolation(t *testing.T) {
	s := Stroke{Points: []Point{{X: 100, Y: 200, Pressure: pointer(0.2)}, {X: 116, Y: 200, TimeMS: 16, Pressure: pointer(0.6)}, {X: 116, Y: 216, TimeMS: 32}}}
	samples := s.Samples()
	if len(samples) != 17 {
		t.Fatalf("samples=%v", samples)
	}
	if samples[4].X != 108 || samples[4].Y != 200 || math.Abs(pressure(samples[4])-0.4) > 1e-9 {
		t.Fatalf("bad interpolation %+v", samples[4])
	}
	if samples[8].X != 116 || samples[8].Y != 200 || samples[8].TimeMS != 16 {
		t.Fatal("lost corner")
	}
	if samples[16].Y != 216 || samples[16].TimeMS != 32 {
		t.Fatal("lost endpoint")
	}
}
func TestDecode(t *testing.T) {
	for _, s := range []string{`{"points":[],"typo":1}`, `{"points":[]} {}`, `{"points":`, strings.Repeat(" ", 1024*1024+1) + `{}`} {
		var stroke Stroke
		if Decode(strings.NewReader(s), &stroke) == nil {
			t.Fatal("accepted invalid JSON")
		}
	}
}

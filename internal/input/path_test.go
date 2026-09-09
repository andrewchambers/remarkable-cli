package input

import (
	"math"
	"testing"
)

func TestNewStrokeTiming(t *testing.T) {
	// A short edge takes one quarter of the time; the longer edge takes three quarters.
	path := []Position{{100, 100}, {200, 100}, {200, 400}}
	s, err := NewStroke(path, 600, .7)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{0, 150, 600} {
		p := s.Points[i]
		if p.TimeMS != want || p.X != path[i].X || p.Y != path[i].Y || pressure(p) != .7 {
			t.Fatalf("bad point: %+v", p)
		}
	}
	pts := s.Samples()
	if pts[75].X != 200 || pts[75].Y != 100 || pts[75].TimeMS != 150 {
		t.Fatal("lost corner during playback")
	}
	last := pts[len(pts)-1]
	if last.X != 200 || last.Y != 400 || last.TimeMS != 600 {
		t.Fatal("lost endpoint")
	}
}

func TestNewStrokeClosedPathAndDuplicates(t *testing.T) {
	path := []Position{{100, 100}, {100, 100}, {200, 100}, {200, 200}, {100, 200}, {100, 100}, {100, 100}}
	s, err := NewStroke(path, 1000, .5)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Points) != 5 {
		t.Fatal("consecutive duplicates not removed")
	}
	for i, p := range s.Points {
		if p.TimeMS != i*250 {
			t.Fatal("incorrect closed-path timing")
		}
	}
	first, last := s.Points[0], s.Points[4]
	if first.X != last.X || first.Y != last.Y {
		t.Fatal("closed path lost")
	}
}

func TestNewStrokeValidation(t *testing.T) {
	valid := []Position{{0, 0}, {Width - 1, Height - 1}}
	for _, tc := range []struct {
		path     []Position
		duration int
		pressure float64
	}{
		{nil, 600, .5}, {valid[:1], 600, .5}, {make([]Position, 10001), 600, .5},
		{[]Position{{1, 2}, {1, 2}}, 600, .5},
		{[]Position{{math.NaN(), 0}, {1, 2}}, 600, .5},
		{[]Position{{0, 0}, {Width, 2}}, 600, .5},
		{[]Position{{0, 0}, {1, math.Inf(1)}}, 600, .5},
		{valid, 0, .5}, {valid, 10001, .5}, {valid, 600, 0}, {valid, 600, 1.1}, {valid, 600, math.NaN()},
		// Rounding must never collapse a corner into the starting or ending timestamp.
		{[]Position{{0, 0}, {.001, 0}, {100, 0}}, 600, .5},
		{[]Position{{0, 0}, {99.999, 0}, {100, 0}}, 600, .5},
	} {
		if _, err := NewStroke(tc.path, tc.duration, tc.pressure); err == nil {
			t.Fatalf("accepted invalid input: %+v", tc)
		}
	}
	for _, duration := range []int{1, 10000} {
		if _, err := NewStroke(valid, duration, 1); err != nil {
			t.Fatal(err)
		}
	}
}

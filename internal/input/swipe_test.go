package input

import (
	"math"
	"testing"
)

func TestSwipeValidation(t *testing.T) {
	base := Swipe{From: Position{0, 0}, To: Position{Width - 1, Height - 1}, DurationMS: 400}
	for _, duration := range []int{1, 400, 5000} {
		s := base
		s.DurationMS = duration
		if err := s.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, change := range []func(*Swipe){
		func(s *Swipe) { s.To = s.From }, func(s *Swipe) { s.From.X = -1 },
		func(s *Swipe) { s.To.X = Width }, func(s *Swipe) { s.To.Y = Height },
		func(s *Swipe) { s.From.Y = math.NaN() }, func(s *Swipe) { s.To.X = math.Inf(1) },
		func(s *Swipe) { s.DurationMS = 0 }, func(s *Swipe) { s.DurationMS = 5001 },
	} {
		s := base
		change(&s)
		if s.Validate() == nil {
			t.Fatalf("accepted invalid swipe: %+v", s)
		}
	}
}

func TestSwipeSamples(t *testing.T) {
	for _, s := range []Swipe{
		{From: Position{100, 200}, To: Position{200, 400}, DurationMS: 17},
		{From: Position{200, 400}, To: Position{100, 200}, DurationMS: 1},
	} {
		pts := s.samples()
		if pts[0].X != s.From.X || pts[0].Y != s.From.Y || pts[0].TimeMS != 0 {
			t.Fatal("lost start")
		}
		last := pts[len(pts)-1]
		if last.X != s.To.X || last.Y != s.To.Y || last.TimeMS != s.DurationMS {
			t.Fatal("lost endpoint")
		}
		for i, p := range pts {
			f := float64(p.TimeMS) / float64(s.DurationMS)
			if math.Abs(p.X-(s.From.X+(s.To.X-s.From.X)*f)) > 1e-9 || math.Abs(p.Y-(s.From.Y+(s.To.Y-s.From.Y)*f)) > 1e-9 {
				t.Fatal("nonlinear path")
			}
			if i > 0 && (p.TimeMS <= pts[i-1].TimeMS || p.TimeMS-pts[i-1].TimeMS > 8) {
				t.Fatal("invalid cadence")
			}
		}
	}
}

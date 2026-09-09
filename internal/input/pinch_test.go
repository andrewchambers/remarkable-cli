package input

import (
	"math"
	"strings"
	"testing"
)

func TestPinchValidation(t *testing.T) {
	base := Pinch{Center: Position{700, 900}, StartDistance: 300, EndDistance: 600, DurationMS: 600}
	for _, duration := range []int{1, 600, 5000} {
		p := base
		p.DurationMS = duration
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
		p.StartDistance, p.EndDistance = p.EndDistance, p.StartDistance
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, change := range []func(*Pinch){
		func(p *Pinch) { p.Center.X = math.NaN() }, func(p *Pinch) { p.Center.Y = Height },
		func(p *Pinch) { p.Center.X = 50 }, func(p *Pinch) { p.Center.X = 1300 },
		func(p *Pinch) { p.StartDistance = 0 }, func(p *Pinch) { p.StartDistance = 19 },
		func(p *Pinch) { p.EndDistance = math.Inf(1) }, func(p *Pinch) { p.StartDistance = math.NaN() },
		func(p *Pinch) { p.EndDistance = p.StartDistance }, func(p *Pinch) { p.DurationMS = 0 },
		func(p *Pinch) { p.DurationMS = 5001 },
	} {
		p := base
		change(&p)
		if p.Validate() == nil {
			t.Fatalf("accepted invalid pinch: %+v", p)
		}
	}
	p := Pinch{Center: Position{701.5, 0}, StartDistance: 20, EndDistance: 1403, DurationMS: 1}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	left, right := p.positions(p.EndDistance)
	if left != (Position{0, 0}) || right != (Position{1403, 0}) {
		t.Fatal("incorrect symmetric endpoints")
	}
	for _, raw := range []string{`{"center":{"x":700,"y":900},"start_distance":300,"end_distance":600,"duration_ms":600,"typo":1}`, `{} {}`, `{"duration_ms":1.5}`} {
		var p Pinch
		if Decode(strings.NewReader(raw), &p) == nil {
			t.Fatal("accepted invalid JSON")
		}
	}
}

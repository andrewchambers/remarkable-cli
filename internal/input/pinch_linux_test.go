//go:build linux

package input

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

// Track slots as an evdev consumer does, recording state at SYN_REPORT.
type touchSnapshot struct {
	ids  [2]int32
	x, y [2]int32
	down bool
}
type trackedTouch struct {
	bytes.Buffer
	slot    int
	state   touchSnapshot
	reports []touchSnapshot
	invalid bool
}

func (w *trackedTouch) Write(data []byte) (int, error) {
	for o := 0; o < len(data); o += 24 {
		typ, code := binary.LittleEndian.Uint16(data[o+16:]), binary.LittleEndian.Uint16(data[o+18:])
		v := int32(binary.LittleEndian.Uint32(data[o+20:]))
		switch typ {
		case 0:
			w.reports = append(w.reports, w.state)
		case evKey:
			if code != btnTouch {
				w.invalid = true
			}
			w.state.down = v != 0
		case evAbs:
			switch code {
			case 47:
				w.slot = int(v)
			case 57:
				w.state.ids[w.slot] = v
			case 53:
				w.state.x[w.slot] = v
			case 54:
				w.state.y[w.slot] = v
			}
		}
	}
	return w.Buffer.Write(data)
}

func TestPinchContacts(t *testing.T) {
	for _, distances := range [][2]float64{{300, 600}, {600, 300}} {
		p := Pinch{Center: Position{700, 900}, StartDistance: distances[0], EndDistance: distances[1], DurationMS: 17}
		x, y := axis{Max: 14030}, axis{Max: 18710}
		w := &trackedTouch{state: touchSnapshot{ids: [2]int32{-1, -1}}}
		if err := playPinch(context.Background(), w, p, x, y); err != nil {
			t.Fatal(err)
		}
		if w.invalid || len(w.reports) != 6 {
			t.Fatalf("invalid reports: %+v", w.reports)
		}
		ids := w.reports[0].ids
		if ids[0] < 0 || ids[1] < 0 || ids[0] == ids[1] {
			t.Fatal("need two distinct contacts")
		}
		for i, ms := range []int{0, 8, 16, 17} {
			r := w.reports[i]
			if r.ids != ids || !r.down {
				t.Fatal("contact changed during pinch")
			}
			d := p.StartDistance + (p.EndDistance-p.StartDistance)*float64(ms)/17
			left, right := p.positions(d)
			if r.x != ([2]int32{scale(left.X, Width, x), scale(right.X, Width, x)}) || r.y != ([2]int32{9000, 9000}) {
				t.Fatalf("incorrect synchronized positions: %+v", r)
			}
		}
		if w.state.ids != ([2]int32{-1, -1}) || w.state.down || w.slot != 0 {
			t.Fatal("contacts not released or slot not restored")
		}
	}
}

func TestPinchCleanup(t *testing.T) {
	p := Pinch{Center: Position{700, 900}, StartDistance: 300, EndDistance: 600, DurationMS: 24}
	for _, at := range []int{1, 2, 3} {
		for _, fail := range []bool{false, true} {
			ctx, cancel := context.WithCancel(context.Background())
			w := &interruptedWriter{cancel: cancel, fail: fail, failAt: at}
			err := playPinch(ctx, w, p, axis{Max: 9620}, axis{Max: 13000})
			cancel()
			expected := error(context.Canceled)
			if fail {
				expected = io.ErrClosedPipe
			}
			if !errors.Is(err, expected) {
				t.Fatalf("got %v, want %v", err, expected)
			}
			var release bytes.Buffer
			releasePinch(&release)
			if !bytes.HasSuffix(w.Bytes(), release.Bytes()) {
				t.Fatal("did not attempt both releases")
			}
		}
	}
	// Failure releasing one slot must not prevent attempting the other release.
	for _, at := range []int{5, 6} {
		w := &interruptedWriter{fail: true, failAt: at}
		if err := playPinch(context.Background(), w, p, axis{Max: 9620}, axis{Max: 13000}); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("lost release failure: %v", err)
		}
		if w.calls != 6 {
			t.Fatal("skipped second release")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var b bytes.Buffer
	if err := playPinch(ctx, &b, p, axis{}, axis{}); !errors.Is(err, context.Canceled) || b.Len() != 0 {
		t.Fatal("canceled pinch sent input")
	}
}

//go:build linux

package input

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"
	"time"
)

func TestFrameEncoding(t *testing.T) {
	var b bytes.Buffer
	if err := frame(&b, key(btnTouch, 1), abs(57, -1)); err != nil {
		t.Fatal(err)
	}
	data := b.Bytes()
	if len(data) != 72 {
		t.Fatalf("frame length %d", len(data))
	}
	if binary.LittleEndian.Uint16(data[16:]) != 1 || binary.LittleEndian.Uint16(data[18:]) != 330 || binary.LittleEndian.Uint32(data[20:]) != 1 {
		t.Fatal("wrong key event")
	}
	if binary.LittleEndian.Uint16(data[40:]) != 3 || binary.LittleEndian.Uint16(data[42:]) != 57 || binary.LittleEndian.Uint32(data[44:]) != 0xffffffff {
		t.Fatal("wrong contact release")
	}
	if !bytes.Equal(data[48:], make([]byte, 24)) {
		t.Fatal("missing SYN_REPORT")
	}
}

type shortWriter struct{}

func (shortWriter) Write(b []byte) (int, error) { return len(b) - 1, nil }
func TestShortWrite(t *testing.T) {
	if !errors.Is(frame(shortWriter{}, key(btnTouch, 1)), io.ErrShortWrite) {
		t.Fatal("ignored partial event write")
	}
}
func TestScale(t *testing.T) {
	a := axis{Min: 10, Max: 9620}
	if scale(0, Width, a) != 10 || scale(Width-1, Width, a) != 9620 {
		t.Fatal("bad endpoints")
	}
	if scale(0.5, 2, axis{Max: 4096}) != 2048 {
		t.Fatal("bad pressure")
	}
}
func TestWaitCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(wait(ctx, time.Hour), context.Canceled) {
		t.Fatal("ignored cancellation")
	}
}

type interruptedWriter struct {
	bytes.Buffer
	calls  int
	cancel context.CancelFunc
	fail   bool
	failAt int
}

func (w *interruptedWriter) Write(b []byte) (int, error) {
	w.calls++
	if w.calls == w.failAt {
		if w.fail {
			return 0, io.ErrClosedPipe
		}
		w.cancel()
	}
	return w.Buffer.Write(b)
}
func TestStrokeReleasesOnInterruption(t *testing.T) {
	for _, failAt := range []int{2, 14, 15} { // during hover, at pen-down, and during movement
		for _, fail := range []bool{false, true} {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			w := &interruptedWriter{cancel: cancel, fail: fail, failAt: failAt}
			s := Stroke{Points: []Point{{X: 100, Y: 100}, {X: 200, Y: 200, TimeMS: 1000}}}
			err := playStroke(ctx, w, s, axis{Max: 9620}, axis{Max: 13000}, axis{Max: 4096})
			if err == nil {
				t.Fatal("interruption ignored")
			}
			var release bytes.Buffer
			frame(&release, key(btnTouch, 0), abs(24, 0))
			frame(&release, key(btnPen, 0))
			if !bytes.HasSuffix(w.Bytes(), release.Bytes()) {
				t.Fatal("did not release contact and pen")
			}
		}
	}
}

// Model evdev's stateful filtering, which was hiding X/Y from pen-down packets.
type filteredPen struct {
	state    map[[2]uint16]int32
	contacts []map[uint16]int32
	reports  []time.Time
}

func (p *filteredPen) Write(data []byte) (int, error) {
	report := map[uint16]int32{}
	down := false
	for o := 0; o < len(data); o += 24 {
		typ, code := binary.LittleEndian.Uint16(data[o+16:]), binary.LittleEndian.Uint16(data[o+18:])
		value := int32(binary.LittleEndian.Uint32(data[o+20:]))
		if typ == 0 {
			continue
		}
		k := [2]uint16{typ, code}
		if p.state[k] == value {
			continue
		}
		p.state[k] = value
		if typ == evAbs {
			report[code] = value
		}
		if typ == evKey && code == btnTouch && value == 1 {
			down = true
		}
	}
	if down {
		p.contacts = append(p.contacts, report)
	}
	p.reports = append(p.reports, time.Now())
	return len(data), nil
}
func TestPenDownIncludesFreshCoordinates(t *testing.T) {
	x, y, pr := axis{Max: 9620}, axis{Max: 13000}, axis{Max: 4096}
	for _, pos := range []Position{{100, 200}, {0, 0}, {Width - 1, Height - 1}} {
		px, py := scale(pos.X, Width, x), scale(pos.Y, Height, y)
		pen := &filteredPen{state: map[[2]uint16]int32{{evAbs, 0}: px, {evAbs, 1}: py}}
		// Starting where the previous stroke ended is the hardest case: both axes
		// are already latched to the exact values we need for the new pen-down.
		s := Stroke{Points: []Point{{X: pos.X, Y: pos.Y}, {X: pos.X, Y: pos.Y, TimeMS: 1}}}
		if err := playStroke(context.Background(), pen, s, x, y, pr); err != nil {
			t.Fatal(err)
		}
		if len(pen.contacts) != 1 {
			t.Fatal("expected one contact")
		}
		got := pen.contacts[0]
		gx, hasX := got[0]
		gy, hasY := got[1]
		if !hasX || !hasY || gx != px || gy != py || got[24] != 2048 {
			t.Fatalf("incomplete initial contact report: %v", got)
		}
		if pen.reports[13].Sub(pen.reports[0]) < 90*time.Millisecond {
			t.Fatal("proximity and contact were not separated")
		}
	}
}
func TestAdjacentSensorCoordinate(t *testing.T) {
	a := axis{Min: 10, Max: 100}
	for _, v := range []int32{10, 50, 100} {
		next := adjacent(v, a)
		if next == v || next < a.Min || next > a.Max {
			t.Fatalf("bad hover coordinate %d -> %d", v, next)
		}
	}
}

func TestSwipeContactSequence(t *testing.T) {
	var b bytes.Buffer
	s := Swipe{From: Position{100, 200}, To: Position{Width - 0.1, Height - 0.1}, DurationMS: 17}
	x, y := axis{Min: 10, Max: 9620}, axis{Min: 20, Max: 13000}
	if err := playSwipe(context.Background(), &b, s, x, y); err != nil {
		t.Fatal(err)
	}
	var reports []map[[2]uint16]int32
	report := map[[2]uint16]int32{}
	data := b.Bytes()
	for o := 0; o < len(data); o += 24 {
		typ, code := binary.LittleEndian.Uint16(data[o+16:]), binary.LittleEndian.Uint16(data[o+18:])
		value := int32(binary.LittleEndian.Uint32(data[o+20:]))
		if typ == 0 {
			reports = append(reports, report)
			report = map[[2]uint16]int32{}
			continue
		}
		if typ == evKey && code == btnPen {
			t.Fatal("sent pen event")
		}
		report[[2]uint16{typ, code}] = value
	}
	if len(reports) != 5 {
		t.Fatalf("expected down, three moves, release; got %d reports", len(reports))
	}
	first := reports[0]
	if first[[2]uint16{evKey, btnTouch}] != 1 || first[[2]uint16{evAbs, 53}] != scale(100, Width, x) || first[[2]uint16{evAbs, 54}] != scale(200, Height, y) {
		t.Fatal("bad initial contact")
	}
	if id, ok := first[[2]uint16{evAbs, 57}]; !ok || id < 0 {
		t.Fatal("missing tracking ID")
	}
	for _, r := range reports[1 : len(reports)-1] {
		if _, ok := r[[2]uint16{evAbs, 57}]; ok {
			t.Fatal("restarted touch during movement")
		}
		if _, ok := r[[2]uint16{evKey, btnTouch}]; ok {
			t.Fatal("changed contact during movement")
		}
	}
	end := reports[len(reports)-2]
	if end[[2]uint16{evAbs, 53}] != x.Max || end[[2]uint16{evAbs, 54}] != y.Max {
		t.Fatal("endpoint not clamped to sensor range")
	}
	var release bytes.Buffer
	frame(&release, abs(47, 0), abs(57, -1), key(btnTouch, 0))
	if !bytes.HasSuffix(data, release.Bytes()) {
		t.Fatal("missing release")
	}
}

func TestSwipeReleasesOnInterruption(t *testing.T) {
	for _, at := range []int{1, 2, 3} {
		for _, fail := range []bool{false, true} {
			ctx, cancel := context.WithCancel(context.Background())
			w := &interruptedWriter{cancel: cancel, fail: fail, failAt: at}
			err := playSwipe(ctx, w, Swipe{From: Position{100, 200}, To: Position{300, 400}, DurationMS: 40}, axis{Max: 9620}, axis{Max: 13000})
			cancel()
			expected := error(context.Canceled)
			if fail {
				expected = io.ErrClosedPipe
			}
			if !errors.Is(err, expected) {
				t.Fatalf("got %v, want %v", err, expected)
			}
			var release bytes.Buffer
			frame(&release, abs(47, 0), abs(57, -1), key(btnTouch, 0))
			if !bytes.HasSuffix(w.Bytes(), release.Bytes()) {
				t.Fatal("contact not released")
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var b bytes.Buffer
	if err := playSwipe(ctx, &b, Swipe{}, axis{}, axis{}); !errors.Is(err, context.Canceled) || b.Len() != 0 {
		t.Fatal("canceled gesture sent input")
	}
}

func TestSwipeReleaseFailure(t *testing.T) {
	w := &interruptedWriter{fail: true, failAt: 3}
	err := playSwipe(context.Background(), w, Swipe{From: Position{100, 200}, To: Position{300, 400}, DurationMS: 1}, axis{Max: 9620}, axis{Max: 13000})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("release failure lost: %v", err)
	}
}

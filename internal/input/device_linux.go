//go:build linux

package input

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"remarkable-cli/internal/capture"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	evKey    = 1
	evAbs    = 3
	btnTouch = 330
	btnPen   = 320
)

type event struct {
	typ, code uint16
	value     int32
}

func abs(code uint16, value int32) event { return event{evAbs, code, value} }
func key(code uint16, value int32) event { return event{evKey, code, value} }

type axis struct{ Value, Min, Max, Fuzz, Flat, Resolution int32 }

func ioctl(f *os.File, req uintptr, p unsafe.Pointer) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), req, uintptr(p))
	if e != 0 {
		return e
	}
	return nil
}
func getAxis(f *os.File, code uint16) (axis, error) {
	var a axis
	err := ioctl(f, uintptr(0x80184540)+uintptr(code), unsafe.Pointer(&a))
	if err == nil && a.Max <= a.Min {
		err = fmt.Errorf("invalid axis %d range", code)
	}
	return a, err
}
func scale(v float64, extent int, a axis) int32 {
	return a.Min + int32(math.Round(v/float64(extent-1)*float64(a.Max-a.Min)))
}
func frame(w io.Writer, events ...event) error {
	// Tatsu is Linux ARM64: timeval is two 64-bit longs, input_event is 24 bytes.
	data := make([]byte, (len(events)+1)*24)
	for i, e := range events {
		o := i * 24
		binary.LittleEndian.PutUint16(data[o+16:], e.typ)
		binary.LittleEndian.PutUint16(data[o+18:], e.code)
		binary.LittleEndian.PutUint32(data[o+20:], uint32(e.value))
	}
	n, err := w.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return err
}
func openDevice(name string) (*os.File, error) {
	paths, err := filepath.Glob("/sys/class/input/event*/device/name")
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		b, e := os.ReadFile(p)
		if e == nil && strings.TrimSpace(string(b)) == name {
			node := filepath.Base(filepath.Dir(filepath.Dir(p)))
			return os.OpenFile("/dev/input/"+node, os.O_RDWR, 0)
		}
	}
	return nil, fmt.Errorf("input device %q not found", name)
}
func idle(f *os.File) error {
	var bits [96]byte
	if err := ioctl(f, 0x80604518, unsafe.Pointer(&bits[0])); err != nil {
		return err
	}
	for _, k := range []int{btnTouch, btnPen} {
		if bits[k/8]&(1<<uint(k%8)) != 0 {
			return fmt.Errorf("input device is in use; lift the pen and fingers before injecting")
		}
	}
	return nil
}
func setup() (*os.File, *os.File, *os.File, error) {
	i, err := capture.DeviceInfo()
	if err != nil {
		return nil, nil, nil, err
	}
	if i.Backend != "tatsu-bgra" || runtime.GOARCH != "arm64" {
		return nil, nil, nil, fmt.Errorf("input currently supports Tatsu firmware 3.27.x on ARM64")
	}
	lock, err := os.OpenFile("/run/remarkable-agent-input.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, nil, nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, nil, nil, fmt.Errorf("another gesture is running: %w", err)
	}
	pen, err := openDevice("Elan marker input")
	if err != nil {
		lock.Close()
		return nil, nil, nil, err
	}
	touch, err := openDevice("Elan touch input")
	if err != nil {
		pen.Close()
		lock.Close()
		return nil, nil, nil, err
	}
	if err = idle(pen); err == nil {
		err = idle(touch)
	}
	if err != nil {
		touch.Close()
		pen.Close()
		lock.Close()
		return nil, nil, nil, err
	}
	return pen, touch, lock, nil
}
func wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func SendTap(ctx context.Context, t Tap) (err error) {
	if err = t.Validate(); err != nil {
		return err
	}
	pen, touch, lock, err := setup()
	if err != nil {
		return err
	}
	defer lock.Close()
	defer pen.Close()
	defer touch.Close()
	x, err := getAxis(touch, 53)
	if err != nil {
		return err
	}
	y, err := getAxis(touch, 54)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, frame(touch, abs(47, 0), abs(57, -1), key(btnTouch, 0))) }()
	if err = frame(touch, key(btnTouch, 1), abs(47, 0), abs(57, int32(time.Now().UnixMilli()%65535)), abs(53, scale(t.X, Width, x)), abs(54, scale(t.Y, Height, y)), abs(48, 10), abs(58, 50)); err != nil {
		return err
	}
	return wait(ctx, time.Duration(t.DurationMS)*time.Millisecond)
}
func SendSwipe(ctx context.Context, s Swipe) error {
	if err := s.Validate(); err != nil {
		return err
	}
	pen, touch, lock, err := setup()
	if err != nil {
		return err
	}
	defer lock.Close()
	defer pen.Close()
	defer touch.Close()
	x, err := getAxis(touch, 53)
	if err != nil {
		return err
	}
	y, err := getAxis(touch, 54)
	if err != nil {
		return err
	}
	return playSwipe(ctx, touch, s, x, y)
}

func playSwipe(ctx context.Context, touch io.Writer, s Swipe, x, y axis) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, frame(touch, abs(47, 0), abs(57, -1), key(btnTouch, 0))) }()
	// Keep one tracking ID for the entire contact. Only its position changes.
	// Clamp fractional edge coordinates to the device's reported axis range.
	touchX := func(v float64) int32 { return min(x.Max, max(x.Min, scale(v, Width, x))) }
	touchY := func(v float64) int32 { return min(y.Max, max(y.Min, scale(v, Height, y))) }
	if err = frame(touch, key(btnTouch, 1), abs(47, 0), abs(57, int32(time.Now().UnixMilli()%65535)), abs(53, touchX(s.From.X)), abs(54, touchY(s.From.Y)), abs(48, 10), abs(58, 50)); err != nil {
		return err
	}
	start := time.Now()
	for _, p := range s.samples()[1:] {
		if err = wait(ctx, time.Until(start.Add(time.Duration(p.TimeMS)*time.Millisecond))); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = frame(touch, abs(47, 0), abs(53, touchX(p.X)), abs(54, touchY(p.Y))); err != nil {
			return err
		}
	}
	return nil
}

func SendStroke(ctx context.Context, s Stroke) (err error) {
	if err = s.Validate(); err != nil {
		return err
	}
	pen, touch, lock, err := setup()
	if err != nil {
		return err
	}
	defer lock.Close()
	defer touch.Close()
	defer pen.Close()
	x, err := getAxis(pen, 0)
	if err != nil {
		return err
	}
	y, err := getAxis(pen, 1)
	if err != nil {
		return err
	}
	pr, err := getAxis(pen, 24)
	if err != nil {
		return err
	}
	return playStroke(ctx, pen, s, x, y, pr)
}

func playStroke(ctx context.Context, pen io.Writer, s Stroke, x, y, pr axis) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, frame(pen, key(btnTouch, 0), abs(24, 0)), frame(pen, key(btnPen, 0))) }()
	first := s.Points[0]
	// evdev drops unchanged ABS values. Hover one sensor unit away so both
	// coordinates are delivered together with contact/pressure in the first
	// drawing report. Otherwise xochitl can begin at the first movement sample.
	if err = frame(pen, key(btnPen, 1), abs(0, adjacent(scale(first.X, Width, x), x)), abs(1, adjacent(scale(first.Y, Height, y), y)), abs(25, 0), abs(26, 0), abs(27, 0)); err != nil {
		return err
	}
	// Keep proximity reports flowing before contact; identical reports are
	// filtered by evdev, so alternate by one raw sensor unit while hovering.
	for i := 0; i < 12; i++ {
		if err = wait(ctx, 8*time.Millisecond); err != nil {
			return err
		}
		hx, hy := scale(first.X, Width, x), scale(first.Y, Height, y)
		if i%2 == 1 {
			hx = adjacent(hx, x)
			hy = adjacent(hy, y)
		}
		if err = frame(pen, abs(0, hx), abs(1, hy)); err != nil {
			return err
		}
	}
	start := time.Now()
	for _, p := range s.Samples() {
		if err = wait(ctx, time.Until(start.Add(time.Duration(p.TimeMS)*time.Millisecond))); err != nil {
			return err
		}
		if err = frame(pen, key(btnTouch, 1), abs(0, scale(p.X, Width, x)), abs(1, scale(p.Y, Height, y)), abs(24, max(1, scale(pressure(p), 2, pr)))); err != nil {
			return err
		}
	}
	return nil
}

func adjacent(v int32, a axis) int32 {
	if v < a.Max {
		return v + 1
	}
	return v - 1
}

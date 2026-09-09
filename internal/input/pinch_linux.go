//go:build linux

package input

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

func SendPinch(ctx context.Context, p Pinch) error {
	if err := p.Validate(); err != nil {
		return err
	}
	pen, touch, lock, err := setup()
	if err != nil {
		return err
	}
	defer lock.Close()
	defer pen.Close()
	defer touch.Close()
	slots, err := getAxis(touch, 47)
	if err != nil {
		return fmt.Errorf("query touch slots: %w", err)
	}
	if slots.Min != 0 || slots.Max < 1 {
		return fmt.Errorf("pinch requires at least two touch slots")
	}
	x, err := getAxis(touch, 53)
	if err != nil {
		return err
	}
	y, err := getAxis(touch, 54)
	if err != nil {
		return err
	}
	return playPinch(ctx, touch, p, x, y)
}

func releasePinch(touch io.Writer) error {
	// Attempt both releases even if one write fails, then restore slot zero for
	// subsequent tap/swipe gestures. Type B slots use -1 to end each contact:
	// https://docs.kernel.org/input/multi-touch-protocol.html
	return errors.Join(
		frame(touch, abs(47, 0), abs(57, -1)),
		frame(touch, abs(47, 1), abs(57, -1), key(btnTouch, 0), abs(47, 0)),
	)
}

func playPinch(ctx context.Context, touch io.Writer, p Pinch, x, y axis) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, releasePinch(touch)) }()
	positionEvents := func(slot int32, pos Position) []event {
		return []event{abs(47, slot),
			abs(53, min(x.Max, max(x.Min, scale(pos.X, Width, x)))),
			abs(54, min(y.Max, max(y.Min, scale(pos.Y, Height, y))))}
	}
	left, right := p.positions(p.StartDistance)
	id := int32(time.Now().UnixMilli() % 65534)
	events := []event{key(btnTouch, 1)}
	for slot, pos := range []Position{left, right} {
		events = append(events, abs(47, int32(slot)), abs(57, id+int32(slot)))
		events = append(events, positionEvents(int32(slot), pos)[1:]...)
		events = append(events, abs(48, 10), abs(58, 50))
	}
	// Both fingers enter in one report, and move together in every later report.
	if err = frame(touch, events...); err != nil {
		return err
	}
	start := time.Now()
	for ms := min(8, p.DurationMS); ; ms = min(ms+8, p.DurationMS) {
		if err = wait(ctx, time.Until(start.Add(time.Duration(ms)*time.Millisecond))); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		distance := p.StartDistance + (p.EndDistance-p.StartDistance)*float64(ms)/float64(p.DurationMS)
		left, right = p.positions(distance)
		events = append(positionEvents(0, left), positionEvents(1, right)...)
		if err = frame(touch, events...); err != nil {
			return err
		}
		if ms == p.DurationMS {
			return nil
		}
	}
}

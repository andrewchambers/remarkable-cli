//go:build !linux

package input

import (
	"context"
	"fmt"
)

func SendTap(context.Context, Tap) error {
	return fmt.Errorf("input injection requires Linux on the tablet")
}
func SendSwipe(context.Context, Swipe) error {
	return fmt.Errorf("input injection requires Linux on the tablet")
}
func SendPinch(context.Context, Pinch) error {
	return fmt.Errorf("input injection requires Linux on the tablet")
}
func SendStroke(context.Context, Stroke) error {
	return fmt.Errorf("input injection requires Linux on the tablet")
}

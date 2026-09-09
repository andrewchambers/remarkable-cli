package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"os/signal"
	"remarkable-cli/internal/buildinfo"
	"remarkable-cli/internal/capture"
	"remarkable-cli/internal/input"
	"syscall"
)

func run() error {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		_, err := fmt.Fprintln(os.Stdout, "remarkable-agent", buildinfo.String())
		return err
	}
	if len(os.Args) == 2 && isHelp(os.Args[1]) {
		fmt.Fprint(os.Stderr, agentHelp)
		return nil
	}
	if len(os.Args) == 3 && isHelp(os.Args[2]) {
		if help, ok := agentCommandHelp[os.Args[1]]; ok {
			fmt.Fprint(os.Stderr, help)
			return nil
		}
		return fmt.Errorf("unknown command %q; run remarkable-agent --help", os.Args[1])
	}
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: remarkable-agent <version|info|screenshot|tap|swipe|pinch|line|stroke>; use --help for details")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	switch os.Args[1] {
	case "pinch":
		var p input.Pinch
		if err := input.Decode(os.Stdin, &p); err != nil {
			return err
		}
		return input.SendPinch(ctx, p)
	case "swipe":
		var s input.Swipe
		if err := input.Decode(os.Stdin, &s); err != nil {
			return err
		}
		return input.SendSwipe(ctx, s)
	case "tap":
		var t input.Tap
		if err := input.Decode(os.Stdin, &t); err != nil {
			return err
		}
		return input.SendTap(ctx, t)
	case "line":
		var l input.Line
		if err := input.Decode(os.Stdin, &l); err != nil {
			return err
		}
		s, err := l.Stroke()
		if err != nil {
			return err
		}
		return input.SendStroke(ctx, s)
	case "stroke":
		var s input.Stroke
		if err := input.Decode(os.Stdin, &s); err != nil {
			return err
		}
		return input.SendStroke(ctx, s)
	case "info":
		i, err := capture.DeviceInfo()
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(i)
	case "screenshot":
		img, err := capture.Capture()
		if err != nil {
			return err
		}
		enc := png.Encoder{CompressionLevel: png.BestSpeed}
		return enc.Encode(os.Stdout, img)
	default:
		return fmt.Errorf("unknown command %q", os.Args[1])
	}
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "remarkable-agent:", err)
		os.Exit(1)
	}
}

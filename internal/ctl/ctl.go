package ctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"remarkable-cli/internal/input"
	"strconv"
	"strings"
	"time"
)

type connection struct {
	host, user, agent, identity string
	port                        int
}

func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func (c connection) command(ctx context.Context, remote string) *exec.Cmd {
	args := []string{"-T", "-o", "BatchMode=yes", "-o", "PreferredAuthentications=publickey", "-o", "StrictHostKeyChecking=yes", "-o", "ConnectTimeout=5", "-o", "ServerAliveInterval=5", "-o", "ServerAliveCountMax=2", "-p", fmt.Sprint(c.port), "-l", c.user}
	if c.identity != "" {
		args = append(args, "-i", c.identity)
	}
	args = append(args, "--", c.host, remote)
	return exec.CommandContext(ctx, "ssh", args...)
}
func (c connection) run(ctx context.Context, remote string, in io.Reader, out, stderr io.Writer) error {
	cmd := c.command(ctx, remote)
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("SSH operation: %w", ctx.Err())
		}
		return fmt.Errorf("SSH operation failed (requires trusted host key and passwordless public-key authentication): %w", err)
	}
	return nil
}

func Run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("remarkablectl", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c := connection{}
	fs.StringVar(&c.host, "host", os.Getenv("REMARKABLE_HOST"), "tablet IP address or SSH host alias (or REMARKABLE_HOST)")
	fs.StringVar(&c.user, "user", "root", "SSH user (capture and input require root)")
	fs.IntVar(&c.port, "port", 22, "SSH port")
	fs.StringVar(&c.identity, "identity", "", "SSH private key path (optional; defaults to SSH config/agent)")
	fs.StringVar(&c.agent, "agent", "/home/root/remarkable-agent", "absolute tablet helper path; also the install-agent destination")
	timeout := fs.Duration("timeout", 30*time.Second, "total SSH operation timeout, including gesture playback (e.g. 15s or 1m)")
	fs.Usage = func() {
		fmt.Fprint(stderr, overviewHelp)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("a command is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	sub := flag.NewFlagSet(fs.Arg(0), flag.ContinueOnError)
	sub.SetOutput(stderr)
	sub.Usage = func() { printCommandHelp(sub, stderr) }
	parse := func() error {
		if err := sub.Parse(fs.Args()[1:]); err != nil {
			return err
		}
		if c.host == "" || strings.HasPrefix(c.host, "-") || strings.ContainsAny(c.host, " \t\r\n\x00") {
			return fmt.Errorf("--host must be an IP address or SSH host alias")
		}
		if c.port < 1 || c.port > 65535 || *timeout <= 0 {
			return fmt.Errorf("invalid port or timeout")
		}
		if !strings.HasPrefix(c.agent, "/") || strings.ContainsAny(c.agent, "\x00\r\n") {
			return fmt.Errorf("--agent must be an absolute remote path")
		}
		return nil
	}
	switch fs.Arg(0) {
	case "pinch":
		center := sub.String("center", "", "required midpoint as x,y in portrait screenshot pixels")
		startDistance := sub.Float64("start-distance", 0, "required initial finger separation in pixels (at least 20)")
		endDistance := sub.Float64("end-distance", 0, "required final finger separation in pixels; larger spreads to zoom in")
		duration := sub.Duration("duration", 600*time.Millisecond, "finger movement duration (1ms..5s, e.g. 600ms or 1s)")
		if err := parse(); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if sub.NArg() != 0 {
			return fmt.Errorf("unexpected pinch arguments")
		}
		pos, err := parsePosition(*center)
		if err != nil {
			return fmt.Errorf("--center: %w", err)
		}
		if *duration < time.Millisecond || *duration > 5*time.Second {
			return fmt.Errorf("pinch duration must be 1ms..5s")
		}
		p := input.Pinch{Center: pos, StartDistance: *startDistance, EndDistance: *endDistance, DurationMS: int(duration.Milliseconds())}
		if err := p.Validate(); err != nil {
			return err
		}
		payload, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return c.run(ctx, quote(c.agent)+" pinch", bytes.NewReader(payload), stdout, stderr)
	case "swipe":
		from := sub.String("from", "", "required finger start coordinate as x,y in portrait screenshot pixels")
		to := sub.String("to", "", "required finger end coordinate as x,y; must differ from --from")
		duration := sub.Duration("duration", 400*time.Millisecond, "finger movement duration (1ms..5s, e.g. 400ms or 1s)")
		if err := parse(); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if sub.NArg() != 0 {
			return fmt.Errorf("unexpected swipe arguments")
		}
		a, err := parsePosition(*from)
		if err != nil {
			return fmt.Errorf("--from: %w", err)
		}
		b, err := parsePosition(*to)
		if err != nil {
			return fmt.Errorf("--to: %w", err)
		}
		if *duration < time.Millisecond || *duration > 5*time.Second {
			return fmt.Errorf("swipe duration must be 1ms..5s")
		}
		s := input.Swipe{From: a, To: b, DurationMS: int(duration.Milliseconds())}
		if err := s.Validate(); err != nil {
			return err
		}
		payload, err := json.Marshal(s)
		if err != nil {
			return err
		}
		return c.run(ctx, quote(c.agent)+" swipe", bytes.NewReader(payload), stdout, stderr)
	case "tap":
		x := sub.Float64("x", -1, "required horizontal screenshot coordinate in pixels (0 <= x < 1404)")
		y := sub.Float64("y", -1, "required vertical screenshot coordinate in pixels (0 <= y < 1872)")
		duration := sub.Duration("duration", 80*time.Millisecond, "contact duration (1ms..2s)")
		if err := parse(); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if sub.NArg() != 0 {
			return fmt.Errorf("unexpected tap arguments")
		}
		t := input.Tap{X: *x, Y: *y, DurationMS: int(duration.Milliseconds())}
		if err := t.Validate(); err != nil {
			return err
		}
		payload, err := json.Marshal(t)
		if err != nil {
			return err
		}
		return c.run(ctx, quote(c.agent)+" tap", bytes.NewReader(payload), stdout, stderr)
	case "line":
		from := sub.String("from", "", "required start coordinate as x,y in portrait screenshot pixels")
		to := sub.String("to", "", "required end coordinate as x,y; must differ from --from")
		duration := sub.Duration("duration", 600*time.Millisecond, "drawing duration (1ms..4s)")
		hold := sub.Duration("hold", 1200*time.Millisecond, "endpoint hold for snapping (800ms..5s)")
		pressure := sub.Float64("pressure", 0.5, "normalized pen pressure (0..1, excluding zero)")
		if err := parse(); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if sub.NArg() != 0 {
			return fmt.Errorf("unexpected line arguments")
		}
		a, err := parsePosition(*from)
		if err != nil {
			return fmt.Errorf("--from: %w", err)
		}
		b, err := parsePosition(*to)
		if err != nil {
			return fmt.Errorf("--to: %w", err)
		}
		l := input.Line{From: a, To: b, DurationMS: int(duration.Milliseconds()), HoldMS: int(hold.Milliseconds()), Pressure: *pressure}
		if err = l.Validate(); err != nil {
			return err
		}
		payload, err := json.Marshal(l)
		if err != nil {
			return err
		}
		return c.run(ctx, quote(c.agent)+" line", bytes.NewReader(payload), stdout, stderr)
	case "stroke":
		points := sub.String("points", "", "required quoted path of space-separated x,y pairs, e.g. '200,300 400,500 600,300'")
		duration := sub.Duration("duration", 600*time.Millisecond, "total movement duration (1ms..10s); time is distributed by segment length")
		pressure := sub.Float64("pressure", 0.5, "constant normalized pen pressure (greater than 0, at most 1)")
		if err := parse(); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if sub.NArg() != 0 {
			return fmt.Errorf("unexpected stroke arguments; quote the entire --points path")
		}
		positions, err := parsePoints(*points)
		if err != nil {
			return err
		}
		if *duration < time.Millisecond || *duration > 10*time.Second {
			return fmt.Errorf("stroke duration must be 1ms..10s")
		}
		s, err := input.NewStroke(positions, int(duration.Milliseconds()), *pressure)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(s)
		if err != nil {
			return err
		}
		if len(payload) > 1024*1024 {
			return fmt.Errorf("stroke exceeds the helper's 1 MiB request limit; use fewer points")
		}
		return c.run(ctx, quote(c.agent)+" stroke", bytes.NewReader(payload), stdout, stderr)
	case "info":
		if err := parse(); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if sub.NArg() != 0 {
			return fmt.Errorf("info takes no positional arguments")
		}
		return c.run(ctx, quote(c.agent)+" info", nil, stdout, stderr)
	case "screenshot":
		output := sub.String("output", "screenshot.png", "local PNG destination path; parent directory must already exist")
		if err := parse(); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if sub.NArg() != 0 {
			return fmt.Errorf("unexpected screenshot arguments")
		}
		if err := savePNG(*output, func(w io.Writer) error { return c.run(ctx, quote(c.agent)+" screenshot", nil, w, stderr) }); err != nil {
			return err
		}
		fmt.Fprintln(stdout, *output)
		return nil
	case "install-agent":
		binaryPath := sub.String("binary", "", "required local path to the Linux ARM64 remarkable-agent binary")
		if err := parse(); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if sub.NArg() != 0 || *binaryPath == "" {
			return fmt.Errorf("install-agent requires --binary PATH")
		}
		f, err := os.Open(*binaryPath)
		if err != nil {
			return err
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil {
			return err
		}
		if !stat.Mode().IsRegular() || stat.Size() == 0 {
			return fmt.Errorf("agent binary must be a nonempty regular file")
		}
		// Receive beside the destination and only replace it after a successful probe.
		remote := "set -eu; dest=" + quote(c.agent) + "; tmp=$(mktemp \"${dest}.XXXXXX\"); trap 'rm -f \"$tmp\"' EXIT HUP INT TERM; cat > \"$tmp\"; chmod 700 \"$tmp\"; \"$tmp\" info >/dev/null; mv -f \"$tmp\" \"$dest\""
		if err := c.run(ctx, remote, f, stdout, stderr); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "Installed", c.agent)
		return nil
	default:
		return fmt.Errorf("unknown command %q", fs.Arg(0))
	}
}

// Save beside the destination; neither SSH failures nor invalid PNGs publish a file.
func savePNG(path string, produce func(io.Writer) error) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".remarkable-screenshot-*.png")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = produce(f); err != nil {
		return err
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		return fmt.Errorf("invalid screenshot PNG: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 4096 || cfg.Height > 4096 {
		return fmt.Errorf("unexpected screenshot dimensions")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err = png.Decode(f); err != nil {
		return fmt.Errorf("incomplete screenshot PNG: %w", err)
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func parsePosition(s string) (input.Position, error) {
	x, y, ok := strings.Cut(s, ",")
	if !ok {
		return input.Position{}, fmt.Errorf("expected x,y")
	}
	px, ex := strconv.ParseFloat(strings.TrimSpace(x), 64)
	py, ey := strconv.ParseFloat(strings.TrimSpace(y), 64)
	if ex != nil || ey != nil {
		return input.Position{}, fmt.Errorf("expected numeric x,y")
	}
	return input.Position{X: px, Y: py}, nil
}

func parsePoints(s string) ([]input.Position, error) {
	fields := strings.Fields(s)
	if len(fields) < 2 || len(fields) > 10000 {
		return nil, fmt.Errorf("--points requires 2..10000 space-separated x,y pairs, e.g. '200,300 400,500'")
	}
	positions := make([]input.Position, len(fields))
	for i, field := range fields {
		p, err := parsePosition(field)
		if err != nil {
			return nil, fmt.Errorf("--points: point %d: %w (use x,y with no spaces around the comma)", i+1, err)
		}
		positions[i] = p
	}
	return positions, nil
}

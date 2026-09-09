package ctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"remarkable-cli/internal/input"
	"strings"
	"testing"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(1, 2, color.RGBA{10, 20, 30, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestSavePNG(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "shot.png")
	data := testPNG(t)
	for _, tc := range []struct {
		name string
		data []byte
		err  error
	}{
		{"ssh failure", data, errors.New("disconnected")},
		{"not PNG", []byte("login banner"), nil},
		{"truncated", data[:len(data)-10], nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := savePNG(dest, func(w io.Writer) error { w.Write(tc.data); return tc.err })
			if err == nil {
				t.Fatal("expected failure")
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 0 {
				t.Fatal("left partial files")
			}
			if err := os.WriteFile(dest, []byte("previous screenshot"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := savePNG(dest, func(w io.Writer) error { w.Write(tc.data); return tc.err }); err == nil {
				t.Fatal("expected replacement failure")
			}
			got, err := os.ReadFile(dest)
			if err != nil || string(got) != "previous screenshot" {
				t.Fatal("damaged existing file")
			}
			entries, err = os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatal("left partial replacement files")
			}
			if err := os.Remove(dest); err != nil {
				t.Fatal(err)
			}
		})
	}
	producer := func(w io.Writer) error { _, err := w.Write(data); return err }
	if err := savePNG(dest, producer); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("old screenshot"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := savePNG(dest, producer); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if !bytes.Equal(got, data) {
		t.Fatal("did not replace existing file")
	}
}
func TestSavePNGConcurrentDestination(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "shot.png")
	want := testPNG(t)
	err := savePNG(dest, func(w io.Writer) error {
		if err := os.WriteFile(dest, []byte("keep"), 0600); err != nil {
			return err
		}
		_, err := w.Write(want)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dest)
	if !bytes.Equal(data, want) {
		t.Fatal("did not replace concurrent destination")
	}
}
func TestShellQuote(t *testing.T) {
	for _, value := range []string{"/home/root/remarkable-agent", "a'b; $(exit 17) `exit 18`\n"} {
		out, err := exec.Command("sh", "-c", "printf %s "+quote(value)).Output()
		if err != nil || string(out) != value {
			t.Fatalf("quote did not round trip: %q %v", out, err)
		}
	}
}
func TestSSHArguments(t *testing.T) {
	c := connection{host: "192.168.0.113", user: "root", port: 22}
	cmd := c.command(context.Background(), "agent screenshot")
	joined := strings.Join(cmd.Args, "|")
	for _, expected := range []string{"BatchMode=yes", "PreferredAuthentications=publickey", "StrictHostKeyChecking=yes", "-T", "--|192.168.0.113|agent screenshot"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("missing %s", expected)
		}
	}
}

func TestLineCLIValidation(t *testing.T) {
	for _, args := range [][]string{
		{"--host", "tablet", "line"},
		{"--host", "tablet", "line", "--from", "1,2,3", "--to", "3,4"},
		{"--host", "tablet", "line", "--from", "NaN,2", "--to", "3,4"},
		{"--host", "tablet", "line", "--from", "1,2", "--to", "3,4", "--hold", "0s"},
	} {
		if Run(args, io.Discard, io.Discard) == nil {
			t.Fatal("accepted invalid line command")
		}
	}
	p, err := parsePosition(" 12.5, 40 ")
	if err != nil || p.X != 12.5 || p.Y != 40 {
		t.Fatal("coordinate parsing failed")
	}
}

func TestHelpWithoutConnection(t *testing.T) {
	t.Setenv("REMARKABLE_HOST", "")
	// No SSH executable is available: help must finish locally before validating
	// the connection, opening an input file, or trying to run the helper.
	t.Setenv("PATH", t.TempDir())
	for _, command := range []string{"", "install-agent", "info", "screenshot", "tap", "swipe", "pinch", "line", "stroke"} {
		for _, helpFlag := range []string{"-h", "--help"} {
			args := []string{helpFlag}
			if command != "" {
				args = []string{command, helpFlag}
			}
			var stdout, stderr bytes.Buffer
			if err := Run(args, &stdout, &stderr); err != nil {
				t.Fatalf("%v: %v", args, err)
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Usage: remarkablectl") {
				t.Fatalf("%v: expected help on stderr only, got stdout=%q stderr=%q", args, stdout.String(), stderr.String())
			}
		}
	}
	if err := Run([]string{"info"}, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "--host") {
		t.Fatalf("ordinary commands must still require a host: %v", err)
	}
}

func TestSwipeCLI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	// Echo stdin as the fake SSH response, and check the remote dispatch.
	script := "#!/bin/sh\nfor last do :; done\n[ \"$last\" = \"'/home/root/remarkable-agent' swipe\" ] || exit 9\nwhile IFS= read -r line || [ -n \"$line\" ]; do printf '%s' \"$line\"; done\n"
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	for _, duration := range []string{"", "1s"} {
		args := []string{"--host", "tablet", "swipe", "--from", " 1100,900 ", "--to", "350,900"}
		if duration != "" {
			args = append(args, "--duration", duration)
		}
		var out bytes.Buffer
		if err := Run(args, &out, io.Discard); err != nil {
			t.Fatal(err)
		}
		var s input.Swipe
		if err := json.Unmarshal(out.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		want := 400
		if duration != "" {
			want = 1000
		}
		if s.From != (input.Position{X: 1100, Y: 900}) || s.To != (input.Position{X: 350, Y: 900}) || s.DurationMS != want {
			t.Fatalf("bad payload: %+v", s)
		}
	}
	for _, flags := range [][]string{
		{}, {"--from", "1,2"}, {"--from", "1,2,3", "--to", "3,4"},
		{"--from", "NaN,2", "--to", "3,4"}, {"--from", "1,2", "--to", "1,2"},
		{"--from", "1,2", "--to", "3,4", "--duration", "5000.1ms"},
		{"--from", "1,2", "--to", "3,4", "--duration", "0s"},
		{"--from", "1,2", "--to", "3,4", "extra"},
	} {
		err := Run(append([]string{"--host", "tablet", "swipe"}, flags...), io.Discard, io.Discard)
		if err == nil || strings.Contains(err.Error(), "SSH operation") {
			t.Fatalf("expected local validation for %v, got %v", flags, err)
		}
	}
}

func TestPinchCLI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	// Echo stdin as the fake SSH response, and check the remote dispatch.
	script := "#!/bin/sh\nfor last do :; done\n[ \"$last\" = \"'/home/root/remarkable-agent' pinch\" ] || exit 9\nwhile IFS= read -r line || [ -n \"$line\" ]; do printf '%s' \"$line\"; done\n"
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	for _, duration := range []string{"", "1s"} {
		args := []string{"--host", "tablet", "pinch", "--center", " 700,900 ", "--start-distance", "300", "--end-distance", "600"}
		if duration != "" {
			args = append(args, "--duration", duration)
		}
		var out bytes.Buffer
		if err := Run(args, &out, io.Discard); err != nil {
			t.Fatal(err)
		}
		var s input.Pinch
		if err := json.Unmarshal(out.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		want := 600
		if duration != "" {
			want = 1000
		}
		if s.Center != (input.Position{X: 700, Y: 900}) || s.StartDistance != 300 || s.EndDistance != 600 || s.DurationMS != want {
			t.Fatalf("bad payload: %+v", s)
		}
	}
	for _, flags := range [][]string{
		{}, {"--center", "700,900"},
		{"--center", "1,2,3", "--start-distance", "300", "--end-distance", "600"},
		{"--center", "NaN,900", "--start-distance", "300", "--end-distance", "600"},
		{"--center", "700,900", "--start-distance", "300", "--end-distance", "300"},
		{"--center", "700,900", "--start-distance", "300", "--end-distance", "600", "--duration", "5000.1ms"},
		{"--center", "700,900", "--start-distance", "300", "--end-distance", "600", "--duration", "0s"},
		{"--center", "700,900", "--start-distance", "300", "--end-distance", "600", "extra"},
	} {
		err := Run(append([]string{"--host", "tablet", "pinch"}, flags...), io.Discard, io.Discard)
		if err == nil || strings.Contains(err.Error(), "SSH operation") {
			t.Fatalf("expected local validation for %v, got %v", flags, err)
		}
	}
}

func TestStrokePointsCLI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	script := "#!/bin/sh\nfor last do :; done\n[ \"$last\" = \"'/home/root/remarkable-agent' stroke\" ] || exit 9\nwhile IFS= read -r line || [ -n \"$line\" ]; do printf '%s' \"$line\"; done\n"
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		flags            []string
		duration, corner int
		pressure         float64
	}{
		{nil, 600, 150, .5},
		{[]string{"--duration", "1200ms", "--pressure", "0.8"}, 1200, 300, .8},
	} {
		args := append([]string{"--host", "tablet", "stroke", "--points", " 100,100\t200,100\n200,400 "}, tc.flags...)
		var out bytes.Buffer
		if err := Run(args, &out, io.Discard); err != nil {
			t.Fatal(err)
		}
		var s input.Stroke
		if err := input.Decode(&out, &s); err != nil {
			t.Fatal(err)
		}
		if err := s.Validate(); err != nil {
			t.Fatal(err)
		}
		if len(s.Points) != 3 || s.Points[1].TimeMS != tc.corner || s.Points[2].TimeMS != tc.duration || s.Points[2].Y != 400 {
			t.Fatalf("bad timing/geometry: %+v", s)
		}
		for _, p := range s.Points {
			if p.Pressure == nil || *p.Pressure != tc.pressure {
				t.Fatal("bad pressure")
			}
		}
	}
	for _, flags := range [][]string{
		{}, {"--file", "old.json"}, {"--points", "1,2"}, {"--points", "1,2 3,4,5"},
		{"--points", "1, 2 3,4"}, {"--points", "NaN,2 3,4"}, {"--points", "1,2 1,2"},
		{"--points", "1,2 3,4", "--duration", "0ms"},
		{"--points", "1,2 3,4", "--duration", "10000.1ms"},
		{"--points", "1,2 3,4", "--pressure", "0"},
		{"--points", "1,2 3,4", "unquoted-extra"},
	} {
		err := Run(append([]string{"--host", "tablet", "stroke"}, flags...), io.Discard, io.Discard)
		if err == nil || strings.Contains(err.Error(), "SSH operation") {
			t.Fatalf("expected local validation for %v, got %v", flags, err)
		}
	}
}

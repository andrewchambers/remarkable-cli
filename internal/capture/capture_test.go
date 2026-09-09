package capture

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestFrameAddress(t *testing.T) {
	// One small allocation followed by an allocation containing the image.
	mem := make([]byte, 0x6000)
	binary.LittleEndian.PutUint64(mem[0x1008:], 0x1002)
	binary.LittleEndian.PutUint64(mem[0x2008:], 0x4002)
	maps, err := parseMaps(strings.NewReader("0-1000 rw-s 0 00:00 0 /dev/dri/card0\n1000-6000 rw-p 0 00:00 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := frameAddress(bytes.NewReader(mem), maps, 15000)
	if err != nil || p != 0x2010 {
		t.Fatalf("pointer=%#x error=%v", p, err)
	}
	for _, header := range []uint64{0, 2, 0x1000, 0xffffffffffffffff, 0x7002} {
		binary.LittleEndian.PutUint64(mem[0x1008:], header)
		if _, err = frameAddress(bytes.NewReader(mem), maps, 15000); err == nil {
			t.Fatalf("accepted invalid chunk %#x", header)
		}
	}
}
func TestFrameAddressRejectsUnrelatedMapping(t *testing.T) {
	for _, maps := range [][]mapping{
		nil,
		{{start: 0, end: 4096, path: "/dev/dri/card0"}},
		{{start: 0, end: 4096, path: "/dev/dri/card0"}, {start: 8192, end: 16384, perms: "rw-p"}},
		{{start: 0, end: 4096, path: "/dev/dri/card0"}, {start: 4096, end: 16384, perms: "rw-p", path: "[heap]"}},
		{{start: 0, end: 4096, path: "/dev/dri/card0"}, {start: 4096, end: 16384, perms: "rw-s"}},
	} {
		if _, err := frameAddress(bytes.NewReader(nil), maps, 100); err == nil {
			t.Fatal("accepted unrelated mapping")
		}
	}
}

func TestFrameAddressMultipleDRMGroups(t *testing.T) {
	// After a reboot, the tablet had an image-sized allocation after the first
	// DRM group and a smaller allocation after the last group. Searching only
	// the last group missed the live screen buffer.
	maps := []mapping{
		{start: 0, end: 0x1000, path: "/dev/dri/card0"},
		{start: 0x1000, end: 0x5000, perms: "rw-p"},
		{start: 0x5000, end: 0x6000, path: "/dev/dri/card0"},
		{start: 0x6000, end: 0xa000, perms: "rw-p"},
	}
	for _, tc := range []struct {
		name          string
		first, second uint64
		want          int64
	}{
		{"screen before last group", 0x4002, 0x1002, 0x1010},
		{"screen after last group", 0x1002, 0x4002, 0x6010},
		{"invalid earlier candidate", 0, 0x4002, 0x6010},
		{"ambiguous candidates", 0x4002, 0x4002, 0},
		{"no matching candidate", 0x1002, 0x1002, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := make([]byte, 0xa000)
			binary.LittleEndian.PutUint64(mem[0x1008:], tc.first)
			binary.LittleEndian.PutUint64(mem[0x6008:], tc.second)
			candidateMaps := append([]mapping(nil), maps...)
			if tc.first == 0x1002 {
				candidateMaps[1].end = 0x2000
			}
			if tc.second == 0x1002 {
				candidateMaps[3].end = 0x7000
			}
			addr, err := frameAddress(bytes.NewReader(mem), candidateMaps, 15000)
			if tc.want == 0 {
				if err == nil {
					t.Fatalf("accepted ambiguous or missing screen at %#x", addr)
				}
			} else if err != nil || addr != tc.want {
				t.Fatalf("pointer=%#x error=%v; want %#x", addr, err, tc.want)
			}
		})
	}
}
func TestBGRA(t *testing.T) {
	img, err := decodeBGRA([]byte{3, 2, 1, 0, 6, 5, 4, 255}, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(img.Pix, []byte{1, 2, 3, 255, 4, 5, 6, 255}) {
		t.Fatalf("wrong colors: %v", img.Pix)
	}
	if _, err = decodeBGRA([]byte{1}, 2, 1); err == nil {
		t.Fatal("accepted truncated screen")
	}
}

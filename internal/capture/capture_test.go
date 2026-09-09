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
	} {
		if _, err := frameAddress(bytes.NewReader(nil), maps, 100); err == nil {
			t.Fatal("accepted unrelated mapping")
		}
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

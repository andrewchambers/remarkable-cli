// Package capture reads xochitl's screen without modifying the process.
package capture

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Info struct {
	Protocol     int    `json:"protocol"`
	Model        string `json:"model"`
	Firmware     string `json:"firmware"`
	Architecture string `json:"architecture"`
	Backend      string `json:"backend,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
}

func DeviceInfo() (Info, error) {
	i := Info{Protocol: 1, Architecture: runtime.GOARCH}
	model, err := os.ReadFile("/sys/devices/soc0/machine")
	if err != nil {
		return i, fmt.Errorf("read device model: %w", err)
	}
	i.Model = strings.TrimSpace(string(model))
	release, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return i, err
	}
	for _, line := range strings.Split(string(release), "\n") {
		if v, ok := strings.CutPrefix(line, "IMG_VERSION="); ok {
			i.Firmware = strings.Trim(v, "\"'")
		}
	}
	// Only enable the layout verified on hardware. Firmware changes can move buffers.
	if i.Model == "reMarkable Tatsu" && strings.HasPrefix(i.Firmware, "3.27.") && runtime.GOARCH == "arm64" {
		i.Backend = "tatsu-bgra"
		i.Width = 1404
		i.Height = 1872
	}
	return i, nil
}

type mapping struct {
	start, end  int64
	perms, path string
}

func parseMaps(r io.Reader) ([]mapping, error) {
	var maps []mapping
	s := bufio.NewScanner(r)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 5 {
			return nil, fmt.Errorf("malformed process mapping")
		}
		ends := strings.Split(fields[0], "-")
		if len(ends) != 2 {
			return nil, fmt.Errorf("malformed address range")
		}
		start, e1 := strconv.ParseInt(ends[0], 16, 64)
		end, e2 := strconv.ParseInt(ends[1], 16, 64)
		if e1 != nil || e2 != nil || start < 0 || end <= start {
			return nil, fmt.Errorf("invalid address range")
		}
		m := mapping{start: start, end: end, perms: fields[1]}
		if len(fields) > 5 {
			m.path = strings.Join(fields[5:], " ")
		}
		maps = append(maps, m)
	}
	return maps, s.Err()
}

// The DRM-adjacent allocation discovery is adapted from goMarkableStream;
// see licenses/goMarkableStream.txt. Bound every step to the anonymous mapping
// and validate allocation sizes before reading any pixel data.
func frameAddress(mem io.ReaderAt, maps []mapping, size int) (int64, error) {
	// DRM mappings can be split into several groups after a reboot. The screen
	// allocation need not follow the last group in address order.
	var addr int64
	var lastErr error
	for n := 1; n < len(maps); n++ {
		prev, m := maps[n-1], maps[n]
		if prev.path != "/dev/dri/card0" || m.start != prev.end || m.path != "" || m.perms != "rw-p" {
			continue
		}
		candidate, err := frameAddressInMapping(mem, m, size)
		if err != nil {
			lastErr = err
			continue
		}
		if addr != 0 {
			return 0, fmt.Errorf("multiple matching screen allocations after DRM mappings")
		}
		addr = candidate
	}
	if addr != 0 {
		return addr, nil
	}
	if lastErr != nil {
		return 0, fmt.Errorf("screen allocation not found after DRM mappings: %w", lastErr)
	}
	return 0, fmt.Errorf("no anonymous private mapping immediately after DRM buffers")
}

func frameAddressInMapping(mem io.ReaderAt, m mapping, size int) (int64, error) {
	for p, steps := m.start, 0; p <= m.end-16 && steps < 4096; steps++ {
		var header [8]byte
		if _, err := mem.ReadAt(header[:], p+8); err != nil {
			return 0, fmt.Errorf("read allocation header: %w", err)
		}
		raw := binary.LittleEndian.Uint64(header[:])
		// These are mmap-backed glibc chunks (IS_MMAPPED flag); size includes header.
		length := int64(raw &^ 7)
		if raw&7 != 2 || length < 16 || length > m.end-p || length%4096 != 0 {
			return 0, fmt.Errorf("unsupported display allocation at %#x", p)
		}
		if length >= int64(size)+16 && length-int64(size)-16 < 4096 {
			return p + 16, nil
		}
		p += length
	}
	return 0, fmt.Errorf("no matching %d-byte screen allocation", size)
}

func xochitlPID() (string, error) {
	paths, err := filepath.Glob("/proc/[0-9]*/comm")
	if err != nil {
		return "", err
	}
	for _, p := range paths {
		b, e := os.ReadFile(p)
		if e == nil && strings.TrimSpace(string(b)) == "xochitl" {
			return filepath.Base(filepath.Dir(p)), nil
		}
	}
	return "", fmt.Errorf("xochitl is not running")
}

func Capture() (*image.RGBA, error) {
	i, err := DeviceInfo()
	if err != nil {
		return nil, err
	}
	if i.Backend == "" {
		return nil, fmt.Errorf("unsupported screenshot layout: model %q, firmware %q, architecture %q", i.Model, i.Firmware, i.Architecture)
	}
	pid, err := xochitlPID()
	if err != nil {
		return nil, err
	}
	f, err := os.Open("/proc/" + pid + "/maps")
	if err != nil {
		return nil, err
	}
	maps, err := parseMaps(f)
	f.Close()
	if err != nil {
		return nil, err
	}
	mem, err := os.Open("/proc/" + pid + "/mem")
	if err != nil {
		return nil, fmt.Errorf("open xochitl memory (requires root): %w", err)
	}
	defer mem.Close()
	size := i.Width * i.Height * 4
	addr, err := frameAddress(mem, maps, size)
	if err != nil {
		return nil, err
	}
	data := make([]byte, size)
	if _, err = mem.ReadAt(data, addr); err != nil {
		return nil, fmt.Errorf("read screen: %w", err)
	}
	return decodeBGRA(data, i.Width, i.Height)
}

func decodeBGRA(data []byte, w, h int) (*image.RGBA, error) {
	if w <= 0 || h <= 0 || w > 4096 || h > 4096 || len(data) != w*h*4 {
		return nil, fmt.Errorf("invalid screen dimensions or buffer length")
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for n := 0; n < len(data); n += 4 {
		img.Pix[n] = data[n+2]
		img.Pix[n+1] = data[n+1]
		img.Pix[n+2] = data[n]
		img.Pix[n+3] = 255
	}
	return img, nil
}

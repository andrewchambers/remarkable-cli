# remarkablectl

Run reMarkable operations over passwordless SSH. Written in Go with no external
Go dependencies. The local `remarkablectl` runs `remarkable-agent` on the tablet
for each operation; the helper exits after returning its result.

## Build

Requires Go 1.24 or newer, Make, and a local OpenSSH client.

```sh
make build
make test
```

Outputs:

- `bin/remarkablectl`: CLI for the computer running the build.
- `bin/remarkable-agent-linux-arm64`: static Linux ARM64 tablet helper.

## Use

The tablet must already allow root SSH access using a key (directly or through
an unlocked SSH agent). Its host key must already be trusted in `known_hosts`.
The CLI uses OpenSSH with public-key authentication, batch mode, and strict
host-key checking; it never prompts for a password or automatically trusts a host.

Install the helper once, and again after rebuilding it:

```sh
./bin/remarkablectl --host 192.168.0.113 install-agent \
  --binary bin/remarkable-agent-linux-arm64

./bin/remarkablectl --host 192.168.0.113 info

./bin/remarkablectl --host 192.168.0.113 screenshot --output screen.png
```

The helper is installed at `/home/root/remarkable-agent` with mode 0700. Installation
uploads to a temporary sibling, checks that the executable can report device info,
and then renames it into place. It does not configure a daemon or restart xochitl.

Connection flags go **before** the subcommand; command flags go after it:

```sh
export REMARKABLE_HOST=192.168.0.113
./bin/remarkablectl screenshot --output screen.png
./bin/remarkablectl --identity ~/.ssh/remarkable --timeout 15s info
./bin/remarkablectl --help
./bin/remarkablectl screenshot --help
```

`--host` also accepts an SSH host alias. Other connection flags are `--user`
(default `root`), `--port` (default `22`), and `--agent` (absolute remote path).
The entire operation has a 30-second timeout by default.

Use `remarkablectl --help` for the command overview and
`remarkablectl <command> --help` for examples, required inputs, defaults, and
limits. Help works without `--host`, an installed helper, or a tablet connection.
For direct tablet-side use, `remarkable-agent --help` and
`remarkable-agent <command> --help` document the helper's JSON input and output.

Screenshots stream as PNG over SSH. The CLI validates the complete PNG before
atomically replacing the destination, including any existing file. Transfer or
decoding failures preserve the previous file and remove the temporary download.
PNG files are created with mode 0600. The destination directory must already
exist. The CLI prints the saved path on success.

## Tap, swipe, pinch, line, and stroke

Input currently supports the same Tatsu firmware 3.27.x device as screenshot
capture. Use coordinates from the **portrait screenshot**, with `(0, 0)` at
the top-left and `(1403, 1871)` at the bottom-right. Landscape orientation is
not yet supported. Positions refer to the screen, including toolbars, rather
than document coordinates; zoom and scrolling affect where ink lands.

```sh
# Tap the second tool shortcut in the visible left toolbar.
./bin/remarkablectl --host 192.168.0.113 tap --x 55 --y 280

# With a notebook open and a drawing tool selected, draw a 400-pixel square.
./bin/remarkablectl --host 192.168.0.113 stroke \
  --points '300,400 700,400 700,800 300,800 300,400' --duration 2400ms
```

`line` draws one segment and then holds its endpoint to request straight-line
snapping from the tablet:

```sh
./bin/remarkablectl --host 192.168.0.113 line --from 200,300 --to 800,600
./bin/remarkablectl --host 192.168.0.113 line --from 200,300 --to 800,600 \
  --duration 600ms --hold 1200ms --pressure 0.5
```

The default movement takes 600 ms, followed by a 1.2-second hold. Tiny horizontal
updates (up to 0.2 pixels) during that hold keep events flowing so xochitl can
recognize it. Movement toward the endpoint uses straight interpolation with no
jitter. Use a drawing tool that supports snapping; verified with the fineliner.
There is no event-level acknowledgement that snapping occurred, and the command
does not select a tool automatically. `--duration` accepts 1 ms to 4 seconds and
`--hold` accepts 800 ms to 5 seconds; a shorter hold may not trigger snapping.

`stroke` traces a quoted list of screen coordinates, with **no automatic snap
hold or jitter**:

```sh
./bin/remarkablectl --host 192.168.0.113 stroke \
  --points '735,640 757,661 810,600' --duration 400ms
```

Select the desired tool first: the pen draws, selection traces a selection, and
selection eraser removes ink within the outline. Supply space-separated `x,y`
pairs with no spaces around commas. Repeat the first point at the end to close
an outline. Adjacent duplicates are ignored; at least two distinct positions
are required. Paths are limited to 10,000 supplied points and your shell's
argument size limit.

`--duration` defaults to 600 ms and accepts 1 ms to 10 seconds. The CLI distributes
time by segment length for approximately constant speed, rounding timestamps
to milliseconds. Fractional milliseconds in the duration are truncated. If points
are too close together to receive distinct timestamps, increase the duration or
use fewer points. `--pressure` applies to the whole stroke, defaults to 0.5,
and must be greater than 0 and at most 1. No JSON file or per-point timing is needed.

Movement interpolates every 2 ms (approximately 500 updates/second), retaining
the supplied corners and endpoints. `line` uses 8 ms sampling and a 1.2-second
snap hold.
Choose `line` for geometric edges and `stroke` for continuous paths. Both use
the same locking, coordinate mapping, validation, and release handling.

`tap` simulates a finger contact for 80 ms by default; use `--duration 120ms`
to change it (1 ms to 2 seconds). Toolbar positions depend on the current UI;
there is no semantic “select tool by name” command yet.

`swipe` moves one finger in a straight path, using 8 ms interpolation and lifting
at the endpoint. It defaults to 400 ms and accepts `--duration` from 1 ms to
5 seconds (fractional milliseconds are truncated). Endpoints describe finger
movement; the current UI determines whether it turns a page, scrolls, drags,
or does nothing. Use screenshot coordinates and avoid unintended UI controls.
This sends single-finger touch input, with no pen contact. Use `pinch` for
two-finger zoom. A successful command confirms that
events were sent, not that the intended navigation occurred.

```sh
# Move the finger left across the page.
./bin/remarkablectl --host 192.168.0.113 swipe --from 1100,900 --to 350,900
# Move the finger upward.
./bin/remarkablectl --host 192.168.0.113 swipe \
  --from 700,1400 --to 700,600 --duration 600ms
```

Rebuild and reinstall the helper before using a newly added command. Direct
`remarkable-agent swipe` requests read one JSON object from stdin, for example:
`{"from":{"x":1100,"y":900},"to":{"x":350,"y":900},"duration_ms":400}`.

Verified on Tatsu firmware 3.27.3.0: a 400 ms leftward swipe from (1100,900)
to (350,900) navigated from page 1 to page 2; a 600 ms reverse swipe returned
to the original drawing on page 1. At 2.0x notebook zoom, vertical swipes between
(700,1400) and (700,600), lasting 600 ms, moved the drawing vertically in both
directions, with corresponding scrollbar movement. Scroll distance is controlled
by the UI and need not match finger travel. Other UI contexts remain unverified.

`pinch` moves two fingers horizontally and symmetrically around a fixed center.
Increasing the distance between them requests zoom-in; decreasing it requests
zoom-out. Distances are the full separation in portrait screenshot pixels,
must be at least 20, and must keep both fingers on-screen. Choose a center on
the page away from toolbars. It defaults to 600 ms, accepts 1 ms to 5 seconds,
and updates both contacts every 8 ms before releasing them. The UI determines
the zoom amount and limits; the command does not set an exact zoom percentage.

```sh
# Zoom in around the middle of the page.
./bin/remarkablectl --host 192.168.0.113 pinch \
  --center 700,900 --start-distance 300 --end-distance 600
# Request the reverse gesture to zoom out.
./bin/remarkablectl --host 192.168.0.113 pinch \
  --center 700,900 --start-distance 600 --end-distance 300
```

The helper reads one JSON object from stdin for `remarkable-agent pinch`:
`{"center":{"x":700,"y":900},"start_distance":300,"end_distance":600,"duration_ms":600}`.
Rebuild and reinstall the helper to enable the command; `pinch --help` describes
all arguments and limits.

Verified on Tatsu firmware 3.27.3.0 with the notebook drawing visible: a 600 ms
pinch centered at (700,900), spreading from 300 to 600 pixels, displayed **2.0x**
zoom. Reversing those distances returned the drawing to its original size and
position. These are observed results for this view, not a guarantee of an exact
zoom factor in every UI context.

Pressure is scaled to the pen's reported range (0–4096 on the tested tablet).
The selected drawing tool determines its appearance and whether pressure has an
effect. The CLI constructs the timed gesture internally and sends it to the
helper over SSH; both client and helper validate it.

Before drawing, the helper sends about 100 ms of hover updates within one raw
sensor unit of the starting position. Pen-down then includes fresh X/Y values,
preventing the missing-start gaps observed with an immediate transition. This
occurs before contact and does not add ink or alter the path's point timing.
See [input implementation notes](docs/input-behavior.md) for the reasoning behind
the hover sequence, sampling intervals, and snap timing.

Input devices are discovered by name, and axis ranges are queried with evdev
ioctls. The helper writes Linux input events to the existing Elan pen and touch
devices, so xochitl processes them like physical input. Gestures are serialized
with a tablet-side lock. The helper refuses to start while a finger is down or
a pen is in proximity, and attempts to release synthetic contacts on completion,
errors, and handled termination signals. Avoid touching or writing during an
injected gesture: physical input arriving afterward can still interfere. A hard
kill or device failure can prevent cleanup.

Verified on the tablet: a finger tap switched from the first tool shortcut to the
second, and the square example drew a closed path on the open notebook page.

## Compatibility

Screenshot capture currently supports **reMarkable Tatsu, ARM64, firmware
3.27.x**, using a 1404 × 1872 BGRA screen buffer. Verified end to end on the
user's device with firmware **3.27.3.0**. Other models and firmware are rejected
with an explicit error; they need separate tested capture backends. `info` can
report a device even when its capture backend is unsupported.

Capture reads xochitl's memory and requires root. It checks anonymous private
mappings immediately following each group of DRM mappings and walks bounded
mmap allocation headers to locate the image. Matches in multiple mappings are
rejected. It does not write process memory. Firmware changes can break this
internal layout, including within the accepted firmware series.
The capture represents the UI buffer; it does not reproduce physical e-ink
refresh artifacts. A screen changing during capture may produce a torn frame.

## Layout and extension points

- `cmd/remarkablectl`: local entry point.
- `internal/ctl`: SSH transport, helper installation, and atomic PNG saving.
- `cmd/remarkable-agent`: tablet command dispatch; stdout is command data,
  stderr is diagnostics, nonzero exit status indicates failure.
- `internal/capture`: device identification, buffer discovery, and pixel decoding.
- `internal/input`: gesture validation, interpolation, and Linux input injection.

`remarkable-agent info` returns JSON including protocol version `1`, device
identity, and available capture backend. `remarkable-agent screenshot` returns
only PNG bytes. New operations can be added as helper commands over the same
SSH transport without opening another network port.

The DRM allocation-discovery approach is adapted from
[goMarkableStream](https://github.com/owulveryck/goMarkableStream), revision
`8857534b585d1a4ee53dfbc32bd8ab1fac01082e`. Its MIT license is preserved in
[licenses/goMarkableStream.txt](licenses/goMarkableStream.txt); distribute that
notice with the binaries.

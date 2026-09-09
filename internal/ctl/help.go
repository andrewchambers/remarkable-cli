package ctl

import (
	"flag"
	"fmt"
	"io"
)

const overviewHelp = `Run reMarkable operations over passwordless SSH.

Usage: remarkablectl [connection flags] <command> [command flags]

Commands:
  install-agent  Install or update the tablet helper from a local binary.
  info           Show device information and compatibility as JSON.
  screenshot     Save the current screen as a local PNG.
  tap            Send a finger tap at a screen position.
  swipe          Move one finger across the screen for navigation or scrolling.
  pinch          Spread or close two fingers to request zoom in or out.
  line           Draw a segment and hold its endpoint to request snapping.
  stroke         Trace one continuous pen path from coordinate pairs.

Connection flags must precede the command; command flags follow it.
Set --host or REMARKABLE_HOST to a tablet IP address or SSH host alias.
SSH requires a trusted host key and passwordless public-key authentication.
Install the helper once, and again after rebuilding it. Each operation runs
the helper briefly; no daemon is installed.

Capture and input support Tatsu ARM64 firmware 3.27.x, in portrait orientation.
Run remarkablectl <command> --help for usage, examples, and command limits.
Help works locally without a host or tablet connection. Errors go to stderr
and return a nonzero exit status.

Examples:
  remarkablectl --host 192.168.0.113 install-agent --binary bin/remarkable-agent-linux-arm64
  remarkablectl --host 192.168.0.113 info
  remarkablectl --host 192.168.0.113 screenshot --output screen.png

Connection flags:
`

const inputHelp = `
Input requires Tatsu ARM64 firmware 3.27.x and root access.
Coordinates use the portrait screenshot: origin at top-left, x from 0 to 1403
and y from 0 to 1871 for whole pixels; fractional positions are accepted inside
the 1404 x 1872 screen. Landscape is unsupported. Positions include toolbars;
page zoom and scrolling change where ink lands.
Gestures run one at a time. Lift your pen and fingers before starting and leave
the screen untouched during playback. The helper attempts contact release on
completion, errors, and handled termination signals. Success produces no output.
`

var commandHelp = map[string]string{
	"pinch": `Spread or close two fingers around a fixed center to request zoom.

Usage: remarkablectl [connection flags] pinch --center X,Y --start-distance PX --end-distance PX [--duration 600ms]

Distances are the full horizontal separation between two fingers in screenshot
pixels. Both fingers move equally on opposite sides of --center; its position
stays fixed. A larger end distance spreads fingers to zoom in; a smaller end
distance brings them together to zoom out. Distances must differ, be at least
20 pixels, and keep both fingers inside the screen at both ends of the gesture.
Choose a center on the page, away from toolbars, using a recent screenshot.
Duration defaults to 600 ms and accepts 1 ms to 5 seconds. Use units such as
600ms or 1s; fractional milliseconds are truncated. Movement samples are 8 ms
apart and both contacts are released afterward. No pen contact is sent.
The current UI determines the zoom amount and limits. These distances request
a relative gesture, not an exact zoom percentage. Success means events were
sent; use screenshot to verify the view. Rotation and landscape are unsupported.

Examples (zoom in, then request the reverse gesture):
  remarkablectl --host 192.168.0.113 pinch --center 700,900 --start-distance 300 --end-distance 600
  remarkablectl --host 192.168.0.113 pinch --center 700,900 --start-distance 600 --end-distance 300
` + inputHelp,
	"swipe": `Move one finger along a straight path across the screen.

Usage: remarkablectl [connection flags] swipe --from X,Y --to X,Y [--duration 400ms]

Use a recent screenshot to choose positions away from unintended UI controls.
Endpoints describe finger movement, not content movement: --from 1100,900
--to 350,900 moves the finger left. The current app decides whether this turns
a page, scrolls, drags an object, or does nothing. This is a single-finger gesture;
use pinch for two-finger zoom.
Duration defaults to 400 ms and accepts 1 ms to 5 seconds, with units such as
400ms or 1s. Fractional milliseconds are truncated. Positions interpolate every
8 ms, then the finger lifts at the exact endpoint. There is no pen contact or
snap hold. Success confirms event delivery, not that navigation occurred.

Examples (the effect depends on the current view):
  remarkablectl --host 192.168.0.113 swipe --from 1100,900 --to 350,900
  remarkablectl --host 192.168.0.113 swipe --from 700,1400 --to 700,600 --duration 600ms
` + inputHelp,
	"install-agent": `Install or update the helper used by every tablet operation.

Usage: remarkablectl [connection flags] install-agent --binary PATH

Build with make build, then upload bin/remarkable-agent-linux-arm64.
The helper is installed at /home/root/remarkable-agent unless --agent is set
before the command. The remote parent directory must exist and be writable.
Installation uploads a temporary sibling, sets mode 0700, runs its info command,
then renames it over the destination. A failed upload or probe preserves the
previous helper. It installs no daemon and does not restart xochitl.
Success prints the installed remote path.

Example:
  remarkablectl --host 192.168.0.113 install-agent --binary bin/remarkable-agent-linux-arm64
`,
	"info": `Show device information and compatibility as JSON to stdout.

Usage: remarkablectl [connection flags] info

Requires an installed helper. Reports protocol version (currently 1), model,
firmware, and architecture. The backend field identifies the supported capture
implementation; width and height describe its screen dimensions. Capture and
input currently use the same device compatibility check. An omitted backend
means these operations are unsupported; info can still succeed.
This command does not change the page. There are no command flags
other than --help.

Example:
  remarkablectl --host 192.168.0.113 info
`,
	"screenshot": `Capture the current tablet screen and save it as a local PNG.

Usage: remarkablectl [connection flags] screenshot [--output PATH]

Requires root and Tatsu ARM64 firmware 3.27.x. Captures the 1404 x 1872 UI buffer
from xochitl's memory, including toolbars. It does not capture physical e-ink
refresh artifacts; a changing screen can produce a torn frame.
The destination directory must exist. An existing destination is replaced
atomically after the complete PNG is validated. Transfer or decoding failures
preserve the previous file. Saved files have mode 0600. Success prints the saved path.

Example (repeat to replace screen.png with a fresh capture):
  remarkablectl --host 192.168.0.113 screenshot --output screen.png
`,
	"tap": `Simulate one finger tap at a screen position.

Usage: remarkablectl [connection flags] tap --x X --y Y [--duration 80ms]

Use a recent screenshot to choose a position. A tap may activate a toolbar or
other UI control; it does not select tools by name. Contact defaults to 80 ms
and accepts 1 ms to 2 seconds. Duration values use units, such as 120ms or 1s.

Example (tap a position in the left toolbar; its action depends on the UI):
  remarkablectl --host 192.168.0.113 tap --x 55 --y 280 --duration 120ms
` + inputHelp,
	"line": `Draw a pen segment, then hold its endpoint to request straight-line snapping.

Usage: remarkablectl [connection flags] line --from X,Y --to X,Y [command flags]

Open a notebook and select a drawing tool that supports snapping (verified with
the fineliner). This adds ink to the current page using the selected tool;
the command does not select a tool. Endpoints must differ.
Movement defaults to 600 ms, followed by a 1.2-second hold. Movement uses 8 ms
sampling; tiny horizontal endpoint updates keep the hold active. A roughly
100 ms hover precedes contact. Success means events were sent; snapping has no
acknowledgement and depends on the tool and hold duration. Use stroke for a
continuous path without the automatic snap hold.
Pressure is normalized: greater than 0, at most 1, default 0.5. Its visual
effect depends on the selected tool. Duration values require units such as ms or s.

Examples:
  remarkablectl --host 192.168.0.113 line --from 200,300 --to 800,600
  remarkablectl --host 192.168.0.113 line --from 200,300 --to 800,600 --duration 600ms --hold 2400ms --pressure 0.5
` + inputHelp,
	"stroke": `Trace one continuous pen path through a list of screen positions.

Usage: remarkablectl [connection flags] stroke --points 'X,Y X,Y ...' [--duration 600ms] [--pressure 0.5]

Quote the whole path. Separate x,y pairs with spaces, with no spaces around
commas. Supply 2 to 10,000 points, including at least two distinct positions.
Repeat the first point at the end to close a shape. Adjacent duplicate points
are ignored. Longer paths may be limited by your shell's argument size limit.

Duration defaults to 600 ms and accepts 1 ms to 10 seconds, with units such as
400ms or 2s. Time is distributed by segment length for approximately constant
speed, rounded to milliseconds. Fractional milliseconds in --duration are
truncated. If points would share a timestamp, increase the duration or supply
fewer points. Corners and endpoints are retained; movement interpolates at
roughly 2 ms intervals. A roughly 100 ms hover precedes the movement duration.
Pressure stays constant, defaults to 0.5, and must be greater than 0 and at most 1.

Select the desired tool first. A drawing tool adds ink; a selection tool traces
a selection, and selection eraser removes ink within the outline. Appearance
and pressure response depend on the tool. No automatic snap hold or jitter is
added; use line to request snapping. No file or per-point timestamps are needed.

Examples:
  remarkablectl --host 192.168.0.113 stroke --points '735,640 757,661 810,600' --duration 400ms
  remarkablectl --host 192.168.0.113 stroke --points '300,400 600,400 600,700 300,700 300,400' --duration 2400ms --pressure 0.5
` + inputHelp,
}

func printCommandHelp(fs *flag.FlagSet, w io.Writer) {
	if help, ok := commandHelp[fs.Name()]; ok {
		fmt.Fprint(w, help)
	} else {
		fmt.Fprintf(w, "Unknown command %q. Run remarkablectl --help for commands.\n", fs.Name())
	}
	fmt.Fprintln(w, "\nConnection flags go before the command; run remarkablectl --help to list them.")
	fmt.Fprintln(w, "\nCommand flags:")
	fs.PrintDefaults()
	fmt.Fprintln(w, "  -h, --help\n    \tshow this help without connecting to the tablet")
}

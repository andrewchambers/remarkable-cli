package main

func isHelp(s string) bool { return s == "-h" || s == "--help" }

const agentHelp = `Run one operation locally on the reMarkable tablet, then exit.

Usage: remarkable-agent <command>
       remarkable-agent <command> --help

Commands:
  version     Show the helper version and Git commit (also --version).
  info        Show device information and compatibility as JSON.
  screenshot  Write the current screen as PNG bytes to stdout.
  tap         Read a finger tap JSON object from stdin and send it.
  swipe       Read a swipe JSON object from stdin and move one finger.
  pinch       Read a pinch JSON object from stdin and move two fingers for zoom.
  line        Read a line JSON object from stdin; draw and hold to request snapping.
  stroke      Read a stroke JSON object from stdin and replay its pen path.

Usually invoked over SSH by remarkablectl. Capture and input require root and
Tatsu ARM64 firmware 3.27.x. Input uses portrait screenshot coordinates.
Command data goes to stdout; help and diagnostics go to stderr. Failures return
a nonzero exit status. Help and version do not access hardware or read stdin.
Input commands accept one JSON object, at most 1 MiB, with no unknown fields.
Use command --help for the payload format. CLI flag defaults are not applied
to direct helper requests; supply the documented timing and pressure fields.
`

const agentInputHelp = `
Requires root and Tatsu ARM64 firmware 3.27.x. Use portrait screen coordinates:
0 <= x < 1404 and 0 <= y < 1872, origin at top-left, including toolbars.
Landscape is unsupported. Lift the pen and fingers before starting and leave
the screen untouched during playback. Gestures are serialized; an active
gesture or physical contact/proximity prevents another gesture from starting.
The helper attempts contact release on completion, errors, and handled signals.
Success produces no stdout; it indicates event delivery, not a verified UI result.
`

var agentCommandHelp = map[string]string{
	"version": `Show the helper version and Git commit to stdout.

Usage: remarkable-agent version
       remarkable-agent --version

Does not access hardware, read stdin, or require root. The info command includes
the same build identifiers as agent_version and agent_commit in its JSON output.
`,
	"pinch": `Spread or close two fingers around a fixed center using JSON from stdin.

Usage: remarkable-agent pinch < pinch.json

Payload example (zoom in):
  {"center":{"x":700,"y":900},"start_distance":300,"end_distance":600,"duration_ms":600}

Supply center, start_distance, end_distance, and duration_ms (integer milliseconds,
1..5000; no helper defaults). Distances are full horizontal finger separation
in screenshot pixels: both must be at least 20 and must differ. Both fingers
must remain inside the screen. They move symmetrically around the fixed center.
Increasing distance requests zoom-in; decreasing it requests zoom-out. The UI
controls the actual zoom amount and limits; success does not confirm a zoom level.
Use a center on the page away from toolbars. Contacts move together every 8 ms,
then both are released. No pen contact is sent. Rotation is unsupported.
` + agentInputHelp,
	"swipe": `Move one finger along a straight path using a JSON object from stdin.

Usage: remarkable-agent swipe < swipe.json

Payload example:
  {"from":{"x":1100,"y":900},"to":{"x":350,"y":900},"duration_ms":400}

Supply distinct from/to positions and duration_ms (integer milliseconds, 1..5000;
no helper default). Endpoints describe finger movement, not content movement.
One contact moves with 8 ms interpolation and lifts at the endpoint; it sends no
pen contact or snap hold. The current UI decides whether to navigate, scroll,
drag, or ignore it. Choose positions using a recent screenshot. Use pinch for
two-finger zoom.
` + agentInputHelp,
	"info": `Show device information and compatibility as JSON to stdout.

Usage: remarkable-agent info

Fields: agent_version, agent_commit, protocol (currently 1), model, firmware,
and architecture. Agent fields identify this helper build; firmware identifies
the tablet OS. The backend
field identifies the supported capture implementation; width and height describe
its screen dimensions. Capture and input currently use the same device
compatibility check. An omitted backend means these operations are unsupported;
info can still succeed. Accepts no stdin or command flags. Does not change the page.

Example:
  remarkable-agent info
`,
	"screenshot": `Write the current 1404 x 1872 screen as PNG bytes to stdout.

Usage: remarkable-agent screenshot > screen.png

Requires root and Tatsu ARM64 firmware 3.27.x. Reads xochitl's UI buffer from
process memory, including toolbars. A changing screen may produce a torn frame;
physical e-ink refresh artifacts are not represented. Accepts no stdin or flags.
Redirect stdout to a file; shell redirection overwrites that file immediately.
Use remarkablectl screenshot for validated, atomic local saving over SSH.
`,
	"tap": `Send one finger contact using a JSON object from stdin.

Usage: remarkable-agent tap < tap.json

Payload example:
  {"x":55,"y":280,"duration_ms":80}

Supply x, y, and duration_ms (integer milliseconds, 1..2000; no duration default).
The position may activate a UI control; its action depends on the current screen.
` + agentInputHelp,
	"line": `Draw a segment and hold its endpoint to request snapping; read JSON from stdin.

Usage: remarkable-agent line < line.json

Payload example (the same values as the CLI timing and pressure defaults):
  {"from":{"x":200,"y":300},"to":{"x":800,"y":600},"duration_ms":600,"hold_ms":1200,"pressure":0.5}

Supply distinct from/to positions, duration_ms (1..4000), hold_ms (800..5000),
and pressure (greater than 0, at most 1). Times are integer milliseconds.
Open a notebook and select a drawing tool that supports snapping, such as the
fineliner. The gesture uses 8 ms sampling, about 100 ms of hover preparation,
and tiny horizontal endpoint updates during the hold. The tool determines the
rendering and pressure effect. Success does not confirm that snapping occurred.
Use stroke for continuous paths without the automatic snap hold.
` + agentInputHelp,
	"stroke": `Replay one continuous pen path using a JSON object from stdin.

Usage: remarkable-agent stroke < stroke.json

Payload example:
  {"points":[{"x":300,"y":400,"t_ms":0},{"x":600,"y":400,"t_ms":600,"pressure":0.5},{"x":600,"y":700,"t_ms":1200}]}

Provide 2..10000 points in at most 1 MiB. Each point has x, y, t_ms, and optional
pressure. Integer times start at zero, increase strictly, and end within 10000 ms.
Pressure defaults to 0.5 and must be greater than 0 and at most 1.
Open a notebook and select a drawing tool first; its rendering and pressure
behavior apply. Positions and pressure interpolate every 2 ms, retaining supplied
corners and times. About 100 ms of hover preparation precedes contact. There is
no automatic snap hold or jitter; use line to request snapping.
` + agentInputHelp,
}

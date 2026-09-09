# Input implementation notes

These choices were tested on reMarkable Tatsu, firmware 3.27.3.0, using the
fineliner in portrait orientation. They describe observed behavior on one
device, firmware, and tool; recheck them when adding other backends.

## Pen contact

Linux evdev filters unchanged coordinates. Repeating the hover position at
pen-down can therefore omit X/Y from the contact report and leave a visible gap
at the start of a stroke. A passive hover delay alone did not fix the gaps.

`playStroke` sends 12 hover updates, 8 ms apart, alternating between the target
and a position one raw sensor unit away. The final hover position differs on
both axes, so pen-down supplies fresh X/Y at the exact requested start. This
adds about 100 ms before contact without drawing ink or changing path timing.
Regression tests model evdev filtering and check fresh coordinates and pressure
at contact, including repeated endpoints and screen corners.

## Stroke sampling

Ordinary strokes interpolate every 2 ms, preserving supplied corners and
endpoints. Tests of a 320-pixel leg drawn in 600 ms after a right-angle turn
reduced maximum centerline deviation from 6.68 pixels at 8 ms spacing to 1.25
pixels at 2 ms spacing. Adding explicit points while keeping the same effective
8 ms interval did not improve the result. These are screenshot measurements,
not measurements of kernel event delivery or guarantees about rendering.

## Line snapping

`line` uses 8 ms interpolation, a default 600 ms movement, and a default 1200 ms
endpoint hold. During the hold, horizontal motion of up to 0.2 pixels keeps
events flowing while xochitl recognizes the snap gesture. Movement toward the
endpoint has no jitter.

Four paired trials with different line geometries produced identical ink masks
with 1200 ms and 2400 ms holds. The 1200 ms default leaves margin above the
shortest tested hold of 800 ms. Reducing movement to 200 ms worsened geometry in
a separate trial, so the default movement remains 600 ms.

Snapping depends on the selected tool and UI state. Evdev does not acknowledge
that a line snapped; successful execution only confirms that events were sent.

# Debugging missing stroke starts

Tested on reMarkable Tatsu, firmware 3.27.3.0, with the fineliner.

The original square's supplied start/end coordinates matched, but the rendered
stroke began after the requested start. Snapping did not repair the missing
segment. Two problems in the proximity-to-contact transition were exposed:

1. The helper sent the initial coordinates during proximity and repeated them
   at contact. An evdev recording showed those unchanged coordinates were
   filtered out of the contact packet. The delivered sequence was:

   ```text
   proximity: BTN_TOOL_PEN=1, ABS_X=2400, SYN_REPORT
   contact:   BTN_TOUCH=1, ABS_PRESSURE=2048, SYN_REPORT
   movement:  ABS_X=2455, SYN_REPORT
   ```

   Y was already latched at the test line's value and was filtered too. Xochitl
   therefore received no fresh position in the initial contact report.

2. A passive 100 ms hover delay still left offsets at the beginning of separately
   snapped lines. Sending small hover position updates before contact eliminated
   the visible joining gaps in the repeated square test. This establishes a
   working input sequence; it does not establish xochitl's internal filtering
   algorithm or explain the remaining curvature of unsnapped paths.

The fix in `playStroke` positions the hovering pen one raw sensor unit away from
its target and sends 12 updates, 8 ms apart, alternating between the target and
that adjacent coordinate. The last hover position differs on both axes, so
pen-down carries fresh X/Y values at the exact requested start. The updates
occur before contact, add about 100 ms per stroke, and draw no extra ink.
The user's movement timeline begins afterward. The path itself still uses
linear interpolation, with no added position noise.

Validation:

- Repeated the side-by-side square example on the device: both paths close.
- Regression test models evdev filtering and requires fresh X, Y, and pressure
  in the initial contact report, including starts at the previous endpoint and
  screen corners.
- Checked cleanup on cancellation and write failures during hover, contact,
  and movement.

The separate endpoint motion used by `line` to request snapping is unchanged.

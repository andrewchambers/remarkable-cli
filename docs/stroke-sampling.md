# Stroke speed and sampling experiment

Tested on the Tatsu, firmware 3.27.3.0, using the black medium fineliner with
no snapping. The hover/contact fix remained enabled. No runtime code or default
sampling interval was changed for this experiment.

## Method

The same vertical segment, (580,1290) to (580,1610), was redrawn at the same
location after erasing each trial. Trials varied duration (600 or 1200 ms) and
requested update interval (8, 4, or 2 ms). For the original 8 ms interval,
we also compared a two-endpoint file with an explicitly sampled 76-point file.
Fourteen isolated-line trials were run, reversing trial order for the second
repeat. Every corresponding repeat produced the same thresholded ink mask.

A second set used an L-shaped stroke: (260,1290) to (580,1290) to (580,1610).
This tests the transition from horizontal to vertical movement that appeared to
cause the square's waviness. Six timing/rate combinations were measured once each.

Screenshots were measured on the vertical leg. For each row from y=1300 through
1600, the center of dark pixels in x=520..629 was calculated. A best-fit line
through those centers provides the RMS and maximum perpendicular deviations
below. This excludes the first and last 10 pixels of the requested leg. These
are raster-image geometric measurements, not an acknowledgement from xochitl
or a timing measurement of kernel event delivery.

## Results

Maximum centerline deviation from the fitted straight line, in screenshot pixels:

| Time per 320-pixel leg | Requested interval | Isolated segment | Segment after a right-angle turn |
| --- | --- | ---: | ---: |
| 600 ms | 8 ms | 0.57 | 6.68 |
| 600 ms | 4 ms | 0.53 | 3.35 |
| 600 ms | 2 ms | 0.54 | 1.25 |
| 1200 ms | 8 ms | 0.53 | 3.35 |
| 1200 ms | 4 ms | 0.54 | 1.25 |
| 1200 ms | 2 ms | 0.49 | 0.49 |

Adding points without changing the effective 8 ms interval produced identical
ink. Reducing the actual interval helped substantially after corners. Halving
speed produced the same ink mask as doubling update frequency at the original
speed in the matching cases. This suggests that distance traveled per input
update matters; it does not establish the tablet's internal rendering algorithm.

The eraser outline used during these trials traveled roughly 1900 pixels/second
with 8 ms interpolation, versus 533 pixels/second for the faster test strokes.
Its larger per-update movement and sharp corners explain why it is a poor visual
comparison with the slower isolated-line tests.

Raw results, including mask hashes:

- [Isolated-line trials](stroke-sampling-results.json)
- [Corner trials](stroke-corner-results.json)

## Demonstration

The left square was redrawn without snapping at 2 ms spacing, keeping the original
600 ms per edge. The right snapped square, star, and semicircle were preserved.
The current CLI can draw the same square on an appropriately positioned blank page:

```sh
remarkablectl --host 192.168.0.113 stroke \
  --points '260,1290 580,1290 580,1610 260,1610 260,1290' --duration 2400ms
```

The experiment originally used a dense JSON file to override the then-default
8 ms interpolation. The current helper interpolates at 2 ms automatically,
and the CLI accepts coordinate pairs and a total duration; `--file` was removed.

## Adopted default

Following these trials, ordinary `stroke` requests now interpolate every 2 ms.
Sparse paths automatically benefit without manually adding intermediate points. `line` carries an internal 8 ms override so its
previously tested sampling and snapping behavior is preserved. The approximately
100 ms hover preparation and the supplied stroke timestamps are unchanged.

# Line command timing

Measured on the connected Tatsu running firmware 3.27.3.0, using the fineliner
in portrait orientation. All trials retained the approximately 100 ms hover
sequence that fixes missing stroke starts.

The CLI default snap hold is now **1200 ms**, down from 2400 ms. Drawing motion
remains 600 ms. This saves 1.2 seconds of intentional waiting per line without
changing its movement samples or the pen-start sequence.

## Paired trials

Each pair drew the same line at the same location, erasing the previous trial
between runs. Screenshots were compared using a thresholded ink mask in a fixed
750 × 300 pixel region containing the line. All four pairs produced identical
ink masks, not just similar straightness measurements. The timing covers the
local `line` process, including SSH and remote execution; erasing and screenshots
are excluded.

| Line | 2400 ms hold | 1200 ms hold | Ink mask |
| --- | ---: | ---: | --- |
| Long reverse diagonal | 3.412 s | 2.248 s | Identical |
| Short diagonal | 3.776 s | 2.224 s | Identical |
| Steep line | 3.483 s | 2.251 s | Identical |
| Horizontal line | 3.462 s | 2.289 s | Identical |

Median observed latency fell from 3.473 s to 2.250 s, approximately 35%.
Individual SSH timings vary. Raw paired measurements and ink-mask hashes are in
[line-timing-results.json](line-timing-results.json).

An initial forward diagonal also had identical pixel count and line-fit residuals
with 800, 1200, 1600, and 2400 ms holds. We chose 1200 ms to leave 400 ms of margin
over the shortest hold tested, rather than using 800 ms as the default.

Reducing motion from 600 ms to 200 ms changed the resulting geometry: the maximum
perpendicular ink-pixel deviation from a fitted line increased from roughly
1.9 px to 5.4 px in that trial, despite a 1600 ms hold. The motion duration was
therefore left at 600 ms.

## Limits and override

These are finite tests on one device, firmware, and tool, not a statistical
reliability guarantee. Evdev provides no acknowledgement of snapping. To restore
the previous timing, use:

```sh
remarkablectl --host 192.168.0.113 line --from 200,300 --to 800,600 --hold 2400ms
```

After changing the default, a four-line square was redrawn on the device and
checked for closure. The unsnapped square and star were preserved, and the
additional timing-test lines were erased.

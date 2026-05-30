# Preview Scroll Test
:::warning
This page exists only to validate editor preview scrolling.
:::

# Intro Section <span style="display:inline-block;width:1.5rem;height:1.5rem;border:1px solid currentColor;vertical-align:middle;"></span>

This synthetic document mixes regular headings, headings with inline HTML markers, and large preview blocks.

# Opening Notes

The goal is to force layout shifts while we click headings in the left editor pane.

<div style="height:18rem;border:1px solid currentColor;display:flex;align-items:center;justify-content:center;">
  Large layout block A
</div>

Scrolling should remain aligned with the chosen section instead of jumping back to the top.

## First Regular Heading

This section is the first regular heading target.

<div style="height:24rem;border:1px solid currentColor;display:flex;align-items:center;justify-content:center;">
  Large layout block B
</div>

## Second Regular Heading

This section adds more height before the later image headings.

<div style="height:16rem;border:1px solid currentColor;display:flex;align-items:center;justify-content:center;">
  Medium layout block C
</div>

This paragraph exists to keep the preview tall and to exercise section changes.

<div style="height:10rem;border:1px solid currentColor;display:flex;align-items:center;justify-content:center;">
  Compact layout block D
</div>

## Third Regular Heading

This heading helps verify scrolling through several plain markdown headings.

<div style="height:20rem;border:1px solid currentColor;display:flex;align-items:center;justify-content:center;">
  Large layout block E
</div>

## Fourth Regular Heading

This section introduces another layout change before the inline-image headings.

<div style="height:8rem;border:1px solid currentColor;display:flex;align-items:center;justify-content:center;">
  Compact layout block F
</div>

## Inline Marker Heading One <span style="display:inline-block;width:1.5rem;height:1.5rem;border:1px solid currentColor;vertical-align:middle;"></span>

This is the first heading that embeds inline HTML.

<div style="height:18rem;border:1px solid currentColor;display:flex;align-items:center;justify-content:center;">
  Large layout block G
</div>

## Inline Marker Heading Two <span style="display:inline-block;width:1.5rem;height:1.5rem;border:1px solid currentColor;vertical-align:middle;"></span>

This section should also align correctly after editor clicks.

## Inline Marker Heading Three <span style="display:inline-block;width:1.5rem;height:1.5rem;border:1px solid currentColor;vertical-align:middle;"></span>

This heading exists to catch regressions in the middle of the sequence.

## Inline Marker Heading Four <span style="display:inline-block;width:1.5rem;height:1.5rem;border:1px solid currentColor;vertical-align:middle;"></span> <span style="display:inline-block;width:1.5rem;height:1.5rem;border:1px solid currentColor;vertical-align:middle;"></span>

This heading uses multiple inline HTML markers on one line.

## Inline Marker Heading Five <span style="display:inline-block;width:1.5rem;height:1.5rem;border:1px solid currentColor;vertical-align:middle;"></span>

This section adds another large block below.

<div style="height:18rem;border:1px solid currentColor;display:flex;align-items:center;justify-content:center;">
  Large layout block H
</div>

This paragraph helps keep the preview tall after the fifth image heading.

## Inline Marker Heading Six <span style="display:inline-block;width:1.5rem;height:1.5rem;border:1px solid currentColor;vertical-align:middle;"></span>

This heading is near the end of the sequence and was useful for regression coverage.

<div style="height:12rem;border:1px solid currentColor;display:flex;align-items:center;justify-content:center;">
  Medium layout block I
</div>

## Final Regular Heading

This final plain heading confirms we can still scroll correctly after multiple image headings.

<div style="height:14rem;border:1px solid currentColor;display:flex;align-items:center;justify-content:center;">
  Final layout block J
</div>

## Last Inline Marker Heading <span style="display:inline-block;width:1.5rem;height:1.5rem;border:1px solid currentColor;vertical-align:middle;"></span>

This is the last heading in the scroll regression fixture.

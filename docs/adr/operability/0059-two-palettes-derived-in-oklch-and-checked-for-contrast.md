# 0059. The UI ships two palettes derived in OKLCH and checked for contrast

**Status:** Accepted ·
**Serves:** [O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer)

## Context

The operator asked for two palettes, one for dark and one for light themes, chosen by palette
selection best practices, with enough differentiation that colors can be told apart and no
harshness or eye strain, and said the dark palette is the one mostly used. Legibility is
[O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer)'s outcome, so the palette is a
design fact rather than a taste.

## Decision

- **Two palettes, dark and light, with the same roles.** Ground, surface, raised, border, text,
  muted, faint, action, restricted, flagged, ok, info, and a chart neutral fill. The tokens are
  stated once in [docs/UI.md](../../UI.md#141-palettes).
- **Derived in OKLCH**, so that within a role set lightness and chroma are held constant and only
  hue varies. Dark uses a warm near-black ground and off-white text, never pure black or pure
  white, with surfaces stepping up in lightness for elevation and accents lighter and less
  saturated than their light-mode counterparts. Light uses a warm off-white ground and dark
  warm-gray text.
- **Checked with WCAG 2 contrast ratios.** Every text and badge color clears 4.5:1 on its surface
  in both modes. Large text, icons, and controls clear 3:1. The derivation script and its report
  live with the mockups at [ui/design/](../../../ui/design/README.md).
- **The categorical chart series are the validated reference palette of the data-visualization
  method**, first six slots, re-checked on these surfaces for lightness band, chroma floor,
  colorblind separation, normal-vision separation, and contrast. Series hues are assigned in fixed
  order and never cycled. Sensitivity and state in charts use the semantic roles and the neutral
  fill, never the series.
- **Color never carries meaning alone.** Every status and sensitivity badge has a label, and text
  wears text colors.

## Alternatives considered

- **A derived six-hue chart series matching the interface palette's hue plan.** Its case was one
  visual system for interface and charts. Rejected because at equal lightness the set failed the
  colorblind-separation check in the validator in both modes and the normal-vision floor in dark.
- **One palette.** No case was tabled. The operator asked for two.

## Consequences

- Any new color enters through the token tables and the derivation, not ad hoc. In light mode
  three chart slots sit under 3:1 on the surface, so charts using them carry visible labels or a
  table view.
- Assumptions about other components: none. The palette is the UI's own.
- No injection. Keeping tokens above their contrast targets stands as review discipline, with the
  derivation script and its report as the instrument, dispositioned in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).

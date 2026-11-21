# SAPIE mark candidates

These are candidates, not final art. G1 picks one direction. Until G1 signs off, nothing here is
shipped as the SAPIE mark. See `plan.md` 13.2.2 and `design/identity.md`.

## Directions

All three keep the fixed concept: one rounded-square field, one keyhole cut, one play triangle. They
differ only in treatment.

| File | Treatment | Reason |
| --- | --- | --- |
| `mark-directions/candidate-a-nested.svg` | Solid violet field, dark keyhole, solid triangle nested in the bowl | Triangle and field share one colour, so it holds at 16px. This is the recommendation. |
| `mark-directions/candidate-b-outline.svg` | Outlined field and keyhole, solid triangle | Lighter and more technical, stronger at 512px, weaker at 16px. |
| `mark-directions/candidate-c-inverted.svg` | Dark field, violet keyhole, dark triangle cut | Inverts the accent so the keyhole carries the mark. |

## Provisional assets

`design/mark.svg`, `design/app-icon.svg`, `design/wordmark.svg`, `design/tray.svg` and the exports in
this folder use direction A as a stand-in. They are provisional pending G1. Do not treat them as
approved identity.

Exports: `sapie-mark-16.svg`, `sapie-mark-22.svg`, `sapie-mark-32.svg`, `sapie-mark-48.svg`,
`sapie-mark-64.svg`, `sapie-mark-128.svg`, `sapie-mark-256.svg`, `sapie-mark-512.svg`.

Every export shares `viewBox="0 0 64 64"`. The pixel size is the `width` and `height` attribute.

## Monogram

`tools/gen-monogram.py APPID` prints a deterministic geometry tile as SVG. The same appid always
produces the same bytes. No font, no network, no invented cover art. Run
`tools/gen-monogram.py --self-test` for the stability check.

## Colour and type

Only `#C5A4FF` and dark neutrals from `design/tokens.css`. No gradient, glow, glass or ember. The
wordmark uses Manrope at weight 700 because the type system caps at 700. The brief calls it
ExtraBold; that wording is resolved at G1.

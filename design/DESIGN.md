# SAPIE launcher design

Reading this as: a native game library for players who already own their games, in the same quiet
instrument-panel language as the web UI, dial ENERGY 2 / RHYTHM 2 / MOTION 2.

This file is the design direction for the Tauri launcher. Colour, type, spacing and radius live in
`design/tokens.css`, copied from `web/src/styles.css`. The launcher must not repeat those values in
its own CSS. The web app will consume the shared file in a later phase.

## Theme decision

Dark only. This is a user decision for 0.4, not an accident of tooling. The launcher is a game
library used in the same dim rooms as the games it starts, and the shared palette is already a dark
instrument panel. Shipping a second theme would mean a second palette, a second WCAG AA contrast
pass and a second set of state visuals for a surface that has no light-mode requirement. There is no
theme toggle and no system-theme following in this phase. If the user reverses the decision, the
toggle and the light token set are a separate phase with their own contrast evidence.

## Colour

All values come from `design/tokens.css`. Each colour has one job.

| Token | Job and reason |
| --- | --- |
| `--bg` | Outer plane. Solid, so the workspace reads as a separate surface without a gradient. |
| `--surface` | Workspace and activity drawer. Neutral enough for dense text. |
| `--surface-quiet` | Rail and detail panel. Recedes without adding a hue. |
| `--text` | Primary text. Measured 13.5:1 on `--surface`. |
| `--muted` | Labels and metadata. Measured 8.69:1 on `--surface`. |
| `--accent` | Ember. Marks focus, active rail item, real live state and the single primary action later. It is never a background wash. |
| `--on-accent` | Dark ink on ember. Measured 9.11:1 on `--accent`. |
| `--status` | Green. Only for a real online state, measured 10.25:1 on `--surface`. |

No glass, no gradient, no glow, no per-card colour. The web glass tokens stay in the shared file
because the web app uses them; the launcher does not consume them. Shadow is used only on the
activity drawer and toasts, the two surfaces that float above the workspace.

## Type

Manrope carries UI text because its compact shapes stay readable in dense rows. IBM Plex Mono is
reserved for paths, appids and other machine values that people compare visually. The launcher has
no bundled font files in this phase, so a machine without Manrope uses the system sans fallback in
the token. That is a known limit, not a claim that the intended face is always present. Bundling the
woff2 files is a later polish task.

Scale is 12/13/14/16/20/28, inherited from the shared tokens. Page titles use -0.02em tracking.
No all-caps decorative labels.

## Layout and identity

The shell is a three-column frame: rail, workspace, detail panel. The activity drawer and toasts sit
above it. This follows the launcher screen map in `plan.md` section 12.2.3. The rail has only the
destinations that exist in this phase: Library and Activity. Friends, Settings and Add game are not
shown until their screens exist, so there is no navigation to nowhere and no dead button.

The identity motif is the square ember "S" mark in the rail and the square monogram placeholder in
the detail panel. Both are type, not invented art. No cover images ship with the launcher.

Radius comes from the shared tokens: 6px on controls, 10px on state surfaces and the drawer, 16px
on the three persistent frames. Hairline edges define boundaries instead of shadows. Spacing uses
the shared 4/8/12/16/20/24/32 scale; panels use 20px, controls and rows use 12px.

## Motion

MOTION 2. One GSAP timeline runs on first shell mount and brings in the rail mark, workspace heading
and detail heading over about 0.3s. It uses `useGSAP` with a scope ref and cleans itself up. When
`prefers-reduced-motion: reduce` is set, the timeline is skipped and CSS transitions are disabled.
There is no looping pulse, no per-card cascade and no fake delay before data appears.

## Honest states

Library data is read from an optional local snapshot on mount. No snapshot means an empty library,
not a sample game list. A malformed snapshot produces an error with a Retry action that re-reads the
same source. Loading is the interval before that read completes. Search gives visible feedback when
nothing matches. The detail panel says when no game is selected. The offline banner uses the
browser's real `navigator.onLine` signal and states that a configured game can still launch from
cached settings. The activity drawer logs launcher events only, with no invented people or actions.

## Audit resolution

Findings from `anti-slop/audit-001-2026-11-20.md`:

| Finding | Resolution in this phase |
| --- | --- |
| 1, R-26 dead Connect control | The egui app is removed. No control claims a network result. Login is M14-c. |
| 2, R-24 web Games nav | Web is out of this phase's scope. The launcher rail only contains Library and Activity. |
| 3, R-37 no launcher direction | This file and `plan.md` section 12.2.11 supply the direction and dials. |
| 4, R-27 no loading state | Library snapshot loading, empty and error states are real surfaces. Long operations arrive with the command layer. |
| 5, R-06 machine font | Shared tokens keep the intended faces with a system fallback. The limit is recorded above, not hidden. |
| 6, R-05 linear wizard | The wizard is gone. Library is the home surface. |
| 7, R-20 no identity | Ember monogram mark, monogram placeholder and ember focus are the recurring motif. |
| 8, R-21 / R-34 fixed theme | Dark-only reason is written above. No toggle is shown. |
| 9, R-31 no written reasons | Reasons are recorded in this file and in `design/tokens.css`. |

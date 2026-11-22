# SAPIE launcher design

Status: the body of this file records the 0.4 Tauri launcher. `design/identity.md` and `plan.md`
section 13 supersede its visual direction for 0.5. The contrast table at the end uses the 0.5
tokens from `design/tokens.css`.

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

## 0.5 contrast measurement

This is a token-pair calculation from `design/tokens.css` using the WCAG relative luminance formula.
It is not a rendered audit. No screen was captured or sampled, because the 0.5 QML components do not
exist yet. A rendered audit belongs to G1 after M22-e. Normal text needs 4.5:1. Large text, 18px or
14px bold and above, needs 3:1.

| Foreground | Background | Values | Ratio | AA normal |
| --- | --- | --- | --- | --- |
| `--text-primary` | `--bg-app` | `#E6E8EB` / `#17191D` | 14.34:1 | pass |
| `--text-primary` | `--bg-sidebar` | `#E6E8EB` / `#14161A` | 14.75:1 | pass |
| `--text-primary` | `--bg-panel` | `#E6E8EB` / `#1D2025` | 13.31:1 | pass |
| `--text-primary` | `--bg-panel-hover` | `#E6E8EB` / `#252930` | 11.89:1 | pass |
| `--text-primary` | `--bg-elevated` | `#E6E8EB` / `#292D34` | 11.26:1 | pass |
| `--text-primary` | `--accent-soft` | `#E6E8EB` / `#342A46` | 10.93:1 | pass |
| `--text-secondary` | `--bg-app` | `#A0A5AE` / `#17191D` | 7.11:1 | pass |
| `--text-secondary` | `--bg-panel` | `#A0A5AE` / `#1D2025` | 6.60:1 | pass |
| `--text-secondary` | `--bg-elevated` | `#A0A5AE` / `#292D34` | 5.59:1 | pass |
| `--text-muted` | `--bg-app` | `#737984` / `#17191D` | 4.02:1 | fail |
| `--text-muted` | `--bg-panel` | `#737984` / `#1D2025` | 3.73:1 | fail |
| `--text-muted` | `--bg-elevated` | `#737984` / `#292D34` | 3.16:1 | fail |
| `--accent` | `--bg-app` | `#C5A4FF` / `#17191D` | 8.49:1 | pass |
| `--accent` | `--bg-panel` | `#C5A4FF` / `#1D2025` | 7.88:1 | pass |
| `--accent` | `--accent-soft` | `#C5A4FF` / `#342A46` | 6.47:1 | pass |
| `--on-accent` | `--accent` | `#17191D` / `#C5A4FF` | 8.49:1 | pass |
| `--success` | `--bg-panel` | `#73C991` / `#1D2025` | 8.16:1 | pass |
| `--warning` | `--bg-panel` | `#E6B86C` / `#1D2025` | 8.90:1 | pass |
| `--danger` | `--bg-panel` | `#E17B7B` / `#1D2025` | 5.69:1 | pass |
| `--info` | `--bg-panel` | `#82B8E8` / `#1D2025` | 7.76:1 | pass |

`--text-muted` is the one failing pair. It clears 3:1 on every background, so it is usable for large
text only. At the 11 to 12px metadata size in `AGENTS-UI.md` it fails. The value is kept because
section 13.2.3 of `plan.md` and `AGENTS-UI.md` both specify `#737984`. G1 must decide whether to
restrict it to large text or raise it to a value such as `#8E95A0`, which measures 4.58:1 on
`--bg-elevated` and 5.41:1 on `--bg-panel`. No shipped surface uses `--text-muted` today: the web
build aliases its legacy `muted` name to `--text-secondary`, and the QML components are not built
yet. The failure is recorded here rather than hidden or silently changed.

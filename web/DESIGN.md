# SAPIE web UI design

Reading this as: a dark LAN game-server control UI for players and hosts, in a liquid-glass
language, dial ENERGY 1 / RHYTHM 2 / MOTION 1.

Dials: ENERGY 1 / RHYTHM 2 / MOTION 1. The UI speaks quietly, varies row weight by content rather
than by decoration, and spends its motion on one short first-load moment.

## Colour

Each colour has one job. Neutral tones carry structure, ember marks the few places that need a
decision.

| Token | Value | Reason |
| --- | --- | --- |
| Background | `#0e1420` | The darkest plane lets the solid workspace sit clearly above it. |
| Surface | `#1b2432` | The work area is neutral enough for dense text and tables. |
| Quiet surface | `#16202e` | Rows and read-only panels recede without adding another hue. |
| Text | `#e9eff7` | High-contrast near-white keeps primary values legible at small sizes. |
| Muted | `#b3c3d6` | A cool secondary tone separates labels from values without a new accent. |
| Ember accent | `#ff9b6a` | Warm ember is reserved for focus, state and the single primary action. |
| On accent | `#16100d` | Dark ink on ember keeps button text readable. |
| Status | `#8fe3b0` | Green is used only for a real online or unlocked state. |
| Glass fill | `rgb(27 36 50 / 68%)` | The two glass panels inherit the surface hue so text stays grounded. |
| Glass edge | `rgb(233 239 247 / 12%)` | A faint edge defines the glass boundary without a glow. |

Measured contrast: text on surface 13.5:1, muted on surface 8.69:1, dark ink on ember 9.11:1,
status on surface 10.25:1.

## Type

| Role | Choice | Reason |
| --- | --- | --- |
| UI face | Manrope | Its compact shapes stay readable in dense rows and form labels. |
| Machine face | IBM Plex Mono | Steam IDs, app IDs and lobby IDs are compared visually, so fixed width helps. |
| Scale | 12/13/14/16/20/28 | The steps are close enough for utility density but still separate headings from metadata. |
| Tracking | `-0.02em` on page titles only | A small tighten gives headings weight without all-caps styling. |

## Radius and surfaces

| Radius | Used for | Reason |
| --- | --- | --- |
| 4px | Status dots and tiny chips | Small elements should not look pill-shaped. |
| 6px | Buttons, inputs, badges, nav items | A tight radius reads as a control, distinct from content. |
| 10px | Rows, panels, states, toasts | Content blocks need a softer edge than controls. |
| 16px | Rail, workspace, detail pane, login card | Only the largest page surfaces get the widest radius. |

Glass is limited to the nav rail and detail pane. Those are the two persistent frames around the
work area, so blur gives them a stable role. Everything else is solid, which keeps rows and tables
easy to read over any wallpaper. The no-blur fallback uses the solid surface. Shadow marks the
few surfaces that float above the work area: the two glass frames, the login card and the toast.

## Icons

The icons are hand-drawn line glyphs in `web/src/components/Icons.tsx`. They share a 24px grid,
20px size, 1.7px stroke and currentColor fill, so they stay consistent without importing a
library look.

| Icon | Reason |
| --- | --- |
| Friends | Two outlined people directly names the friends list. |
| Games | A gamepad directly names the games route. |
| Profile | A single person directly names the account route. |
| Live | A pulse line fits live sessions without a broadcast cliche. |
| Lock | A padlock explains that the pane cannot change data. |
| Eye / Eye off | The pair makes password visibility obvious and reversible. |
| S mark | The product initial is a text placeholder, not an invented logo asset. |

## Spacing and hierarchy

Spacing runs on 4/8/12/16/20/24/32. Rows use 12px vertical padding, panels 16px and page surfaces
20px, which keeps the utility density high without crowding. Hierarchy comes from size and weight:
friend and lobby names are 16px bold, body values are 14px, metadata is 12px muted mono. Empty,
loading and error states use the same 10px radius surface so failures read as part of the same
system.

## Motion

The GSAP timeline in `Shell.tsx` runs once on first authenticated mount. It brings in the rail
mark, workspace heading and detail heading over roughly 0.3s. Nav icons use a `contextSafe`
tween on click so event-created motion is reverted with the component. `useGSAP` receives a scope
ref and handles cleanup. When `prefers-reduced-motion: reduce` is set, the timeline and pulse are
skipped. There are no per-card reveals, no fake delays and no looping animation.

## Theme

Dark only. There is no light theme and no theme toggle in 0.4.

Reason: this is a LAN control surface for a self-hosted game server. It is read beside a running
game, often in a dim room, and the product has one authored palette. A second light palette would
split the single token source in `design/tokens.css`, add a preference to persist and maintain, and
needs its own contrast and state verification. The user chose dark-only for 0.4, so the web UI
consumes the one dark token set and spends the saved effort on real social and session flows.
The OS `color-scheme` stays `dark` in the tokens. There is no setting claiming a toggle exists.

# SAPIE identity

Source of truth for the name, mark, palette, type, voice, motion and cover rules. This record follows
`plan.md` section 13.2. `AGENTS-UI.md` is the visual base. The overrides at the end of this file are
the only places where 0.5 departs from it.

Reading this as: a game library for players who already own their games, in a quiet dark
instrument-panel language, dial ENERGY 2 / RHYTHM 2 / MOTION 2.

## Name and positioning

| Choice | Reason |
| --- | --- |
| The name is SAPIE, uppercase on every product surface. | One fixed shape is easier to recognise than a styled case. |
| Do not expand the acronym in the UI. The README may explain it once. | The explanation belongs in docs, not in a navigation rail. |
| Family: SAPIE for Windows, SAPIE for Linux, SAPIE Server, SAPIE Web. | One name with four honest jobs and no invented product tiers. |
| Public tagline: Your games. Your server. | Two short sentences state the ownership the product is about. |
| Descriptor: Old Steam games, no Steam client. | It says what the product does without a feature list. |
| Promise: you own the game, the machine and the server. | It names the dependency SAPIE removes and nothing beyond it. |
| Tone is quiet and factual. | The product removes a client, it does not sell a lifestyle. |

## Mark and wordmark

| Choice | Reason |
| --- | --- |
| Mark concept: a keyhole with a play triangle in the negative space. | Unlock and play are the two things the product actually does. |
| One rounded square field, one keyhole cut, one triangle cut. | Three primitives keep the mark legible at 16px and at 512px. |
| No gamepad, robot, sparkle or diamond. | Those shapes describe other products and borrowed genres. |
| Wordmark: SAPIE in Manrope ExtraBold, tight tracking, mark to the left. | The wordmark stays text so it inherits the one type decision. |
| App icon: mark on the brand background, one accent, no gradient or glow. | The icon must match the flat token language of the UI. |
| Tray icon: the silhouette at 16 and 22px, monochrome on Linux where asked. | Tray renderers control colour themselves and need a clean silhouette. |
| Monogram fallback: deterministic geometric tile derived from the appid. | The same appid always gets the same tile, with no invented cover art. |
| The mark is a G1 artifact. Do not generate final art before G1 approves the direction. | R-23 requires confirmation before an asset becomes final. |

The final art itself is out of this record's scope. It is M22-c, after G1 chooses a direction.

## Palette

Values live in `design/tokens.css`. The table names the job of each choice.

| Token | Value | Reason |
| --- | --- | --- |
| `--bg-app` | `#17191D` | A dark gray, not black, so panels can separate without a gradient. |
| `--bg-sidebar` | `#14161A` | Slightly darker than the workspace so the persistent rail recedes. |
| `--bg-panel` | `#1D2025` | Neutral enough for dense rows and cover captions. |
| `--bg-panel-hover` | `#252930` | One step brighter reads as hover without a new hue. |
| `--bg-elevated` | `#292D34` | The top surface for dialogs and menus only. |
| `--border-subtle` | `#2A2E35` | A hairline edge defines boundaries without a shadow. |
| `--border-strong` | `#383E47` | A stronger edge is reserved for focus containers. |
| `--text-primary` | `#E6E8EB` | Near-white stays legible at 12px on every dark surface. |
| `--text-secondary` | `#A0A5AE` | One dimmer step separates labels from values without a new accent. |
| `--text-muted` | `#737984` | Metadata is dimmer again. See the contrast note in `design/DESIGN.md`. |
| `--accent` | `#C5A4FF` | Violet marks active navigation, focus, selection and the primary action. |
| `--accent-soft` | `#342A46` | A quiet violet fill marks a selected row without a wash. |
| `--on-accent` | `#17191D` | Dark ink on violet keeps primary action text readable. |
| `--success` | `#73C991` | Green is reserved for a real online or success state. |
| `--warning` | `#E6B86C` | Amber is reserved for away and warning states. |
| `--danger` | `#E17B7B` | Red is reserved for errors and destructive actions. |
| `--info` | `#82B8E8` | Blue is reserved for informational status. |

One accent, four status colours, no gradients, no glow, no glass. Status is never the only signal;
a dot is paired with a text label. The ember `#ff9b6a` is retired as identity. A warm tone may return
as a warning status only, after a separate decision.

## Type

| Choice | Reason |
| --- | --- |
| UI face: Manrope, with the platform stack as fallback. | It is compact in dense rows and already in the product. |
| Machine face: IBM Plex Mono for paths, appids, SteamIDs, lobby IDs and logs. | Fixed width helps people compare values visually. |
| No remote font import. TTF or OTF in the Qt resource file, woff2 in the web build. | Both apps must render the intended face on a clean machine. |
| Scale: 12, 13, 14, 16, 20, 24. | Six close steps separate metadata from titles without marketing sizes. |
| Page titles 20 to 24, card titles 13 to 14, metadata 11 to 12. | The range matches a desktop library rather than a landing page. |
| Weights 400, 500, 600, 700. No 800 or 900. | Heavy weights break the quiet tone in dense rows. |

Qt cannot load woff2, so the Qt bundle carries TTF or OTF. That is a packaging constraint, not a
second type system.

## Voice

Short, factual, second person. The user is you. The server is your server. The product does not brag
and does not apologize.

| Instead of | Write | Reason |
| --- | --- | --- |
| Get Started | Add a game | It names the action the control performs. |
| Seamless experience | Your server, your games | It states a fact instead of a claim. |
| Oops! Something went wrong | The server did not answer. Check the address and try again. | It says what failed and what to do next. |
| Unlock your gaming potential | Play the games you already own | It describes the product without hype. |
| AI-powered | delete the line | There is no such feature to describe. |

No em dash. No exclamation mark in a system message. No fake numbers, no fake testimonials and no
fake players. Errors name the cause and one next step. A placeholder looks like a placeholder.

## Motion

| Choice | Reason |
| --- | --- |
| 120 to 180ms on hover, 140 to 200ms on menus and dialogs. | Short transitions report state without slowing a library browse. |
| Ease-out entering, ease-in leaving. | The pair reads as a response to the user's action. |
| One first-mount entrance in the launcher. | A single moment gives the shell weight on open. |
| No per-card cascade, bounce, spring or looping background. | Repeated motion competes with cover art. |
| Respect the OS reduced-motion setting, with an in-app fallback. | Qt exposes no QML property for it, so the platform layer reads it. |
| No sound. | A launcher that beeps is a toy. |

## Art and covers

| Choice | Reason |
| --- | --- |
| Cover art is the main colour source. Portrait 2:3, object-fit cover. | Art carries the identity in a library, not the chrome. |
| Missing art uses the monogram tile. | A deterministic tile is honest where a store cover would be invented. |
| The user may set a cover from a local image. Store the path and a cached thumbnail. | No remote URL is needed and no art is fabricated. |
| No invented game metadata. If the server does not know the name, show the executable name. | Real data is the only data this product shows. |
| No glass, gradient wash or fake Mica behind covers. | Art and text need a flat surface to read against. |

## GOG inspiration boundary

GOG Galaxy 2.0 is a reference for density, contrast and the game-library browsing pattern. It is not
a template. The layout language may be inspired by it. The mark, type, voice and monogram system must
be SAPIE's. The clone test: swap the logo and the product name. If the result could be GOG, Steam or
Heroic, the identity has failed and goes back to G1.

## Deliberate overrides of AGENTS-UI.md

| Override | Reason |
| --- | --- |
| Manrope instead of the suggested Inter stack. | Inter is the default AI choice and would erase the one type decision SAPIE already owns. |
| No glass anywhere, including the surfaces that use it today. | A native Qt scene graph renders flat surfaces more crisply than simulated acrylic. |
| Dark only, with no light token set or theme toggle. | The library is used in the same dim rooms as the games it starts, so a second palette and contrast pass buy nothing. |
| No store, download or install entry. Games are added from disk. | No such feature exists and no dead control may be shown. |
| No fake art and no invented game metadata. | Covers and names must come from the user or the server. |
| Status tokens keep their semantic names and the ember is retired. | One accent plus four statuses is easier to audit than a warm brand accent and a separate warning colour. |

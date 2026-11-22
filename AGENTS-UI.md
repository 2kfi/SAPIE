# AGENTS-UI.md — SAPIE UI Design Direction

## 1. Design Goal

Build SAPIE's web UI as a desktop game-library client inspired by **GOG Galaxy 2.0's library experience**.

It should feel like a polished PC launcher:
- Dark, low-glare interface.
- A compact, persistent navigation rail/sidebar.
- Game-cover art as the primary visual content.
- Dense but readable layouts.
- Subtle borders, restrained shadows, and small controls.
- UI chrome should be quiet; content should visually dominate.
- Avoid the generic “admin dashboard” look.

This is a **visual direction**, not a literal GOG Galaxy reproduction. Use the overall layout language, density, contrast, and game-library browsing patterns as inspiration while keeping SAPIE's own identity.

SAPIE's UI should support its actual features: account, friends, online presence, friend requests, invites, and lobbies. Do not invent storefront, purchase, installation, or game-discovery functionality.

---

## 2. Overall Application Frame

### Desktop-first
Design primarily for a desktop window, approximately 1280×720 and above. The UI should scale down gracefully to smaller windows, but desktop is the primary target.

### Application frame
- Full-window app; avoid marketing-site layouts.
- Persistent left sidebar, approximately **164 px** wide at a 1440 px desktop baseline.
- Main content occupies all remaining width.
- Main content may scroll independently from the sidebar.
- Use a very subtle 1 px divider between sidebar and main workspace.
- Keep the top-level frame visually flat and cohesive, like a desktop client.

### Visual hierarchy
1. Game artwork / lobby / friend content.
2. Page title and active section.
3. Primary action.
4. Secondary controls, filters, status indicators.
5. Decorative UI details.

---

## 3. Color Palette

Use a restrained dark palette. Avoid pure black backgrounds and overly bright neon accents.

### Core colors

| Token | Hex | Usage |
|---|---|---|
| `--bg-app` | `#17191D` | Main application background |
| `--bg-sidebar` | `#14161A` | Sidebar background |
| `--bg-panel` | `#1D2025` | Panels, cards, dropdowns |
| `--bg-panel-hover` | `#252930` | Hovered cards / rows |
| `--bg-elevated` | `#292D34` | Popovers, menus, elevated UI |
| `--border-subtle` | `#2A2E35` | Hairline separators |
| `--border-strong` | `#383E47` | Stronger borders / focus containers |
| `--text-primary` | `#E6E8EB` | Main text |
| `--text-secondary` | `#A0A5AE` | Secondary text |
| `--text-muted` | `#737984` | Muted metadata |
| `--text-disabled` | `#555B65` | Disabled controls |
| `--accent` | `#C5A4FF` | Primary accent; muted violet |
| `--accent-hover` | `#D2B8FF` | Accent hover |
| `--accent-pressed` | `#A986E8` | Accent pressed |
| `--accent-soft` | `#342A46` | Accent tint / selected background |
| `--success` | `#73C991` | Online / success |
| `--warning` | `#E6B86C` | Warnings / away |
| `--danger` | `#E17B7B` | Errors / destructive actions |
| `--info` | `#82B8E8` | Informational status |

### Color rules
- Backgrounds should be dark gray, not absolute black.
- Accent color should be used sparingly for active navigation, focus, selected states, and primary actions.
- Do not turn every button purple.
- Online status should use green, but do not rely on color alone: include text or a status icon.
- Avoid gradients as a default. A subtle gradient may be used over artwork or in a special hero area, never as a general UI background.
- No glassmorphism-heavy styling.
- No large glowing neon outlines.

---

## 4. Typography

Use a modern, neutral sans-serif UI font. Prefer the platform's native UI font stack unless the project already includes a suitable font.

Suggested stack:
`Inter, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif`

### Typography scale
- Page title: **20–24 px**, semibold.
- Section title: **15–17 px**, semibold.
- Card title: **13–14 px**, medium.
- Body text: **13–14 px**, regular.
- Sidebar labels: **12–13 px**.
- Metadata / status: **11–12 px**.
- Tiny icon labels: **10–11 px**, use sparingly.

Use modest font weights. Avoid oversized headings that make the app feel like a website landing page.

---

## 5. Sidebar

### Structure
The sidebar is fixed/persistent and contains these groups:

**Top / app area**
- Small SAPIE logo or icon.
- Optional compact profile/account affordance.
- Keep this area small; do not create a large branded banner.

**LIBRARY**
- Library / All Games (if a game collection view exists).
- Recent / Recently played (only if the data exists).
- Installed (only if SAPIE can actually determine this).
- Downloads (only if implemented).

**SAPIE**
- Friends.
- Requests / Pending (show a small count badge if there are pending requests).
- Lobbies.

**SYSTEM**
- Settings.
- About / Help, if implemented.

Do not display dead navigation entries. If a feature is not implemented, omit it rather than showing a disabled fake destination.

### Sidebar styling
- Width: 160–180 px.
- Background: `--bg-sidebar`.
- Right border: `1px solid --border-subtle`.
- Top padding: 12–16 px.
- Navigation rows: 30–34 px high.
- Horizontal padding: 10–12 px.
- Icon size: 14–16 px.
- Label size: 12–13 px.
- Muted section labels: uppercase, 10–11 px, medium weight, slightly increased letter spacing.
- Group spacing: 14–20 px.
- Keep icon and label aligned consistently.

### Navigation states
- Default: transparent background; muted text and icon.
- Hover: subtle `--bg-panel-hover`.
- Active: `--accent-soft` background, accent-colored icon, primary text.
- Active item should have a subtle rounded rectangle (4–6 px radius).
- Avoid thick left bars or large pills; selected state should be quiet.

---

## 6. Main Workspace Header / Toolbar

A compact toolbar spans the top of the main content.

### Layout
- Height: approximately 42–48 px.
- Left side: back/forward navigation if useful, then page context/title.
- Middle: page-specific filters, sorting, view controls.
- Right side: search, optional utility controls, account/avatar.
- Align controls vertically and keep them compact.

### Toolbar controls
- Icon buttons: approximately 28–32 px square.
- Small text buttons: 28–32 px high.
- Search: compact, around 160–220 px wide on desktop; may expand when focused.
- Filter controls should be simple dropdowns, not oversized form fields.
- Use small gaps (6–10 px) between controls.
- A 1 px border or subtle separator may divide toolbar from content.

### Interaction styling
- Default icon: `--text-secondary`.
- Hover: `--text-primary` with subtle panel background.
- Active/toggled filter: accent tint background.
- Focus: visible 1–2 px accent outline with a small offset.
- Disabled: `--text-disabled`.

---

## 7. Library View — Visual Reference Layout

The library is the main visual reference for the Galaxy-inspired look.

### Grid
- Main content starts approximately 8–12 px from the left edge of the content region.
- Use a responsive grid of cover art.
- Desktop target: around **5 columns** at 1280–1440 px main-window width, depending on sidebar and window size.
- Cards should preserve a portrait cover aspect ratio, approximately **2:3**.
- Gaps: 12–16 px horizontally and vertically.
- Cards should be large enough for cover artwork to be recognizable.
- Grid should fill the available width without stretching cover images.
- Use `object-fit: cover` for cover art.
- Keep all cards aligned to a consistent grid.

### Artwork
- Artwork is the main color and texture source.
- Do not place large labels, buttons, or overlays over the artwork by default.
- Avoid generic placeholder gradients where real cover artwork is available.
- For missing artwork, use a neutral dark placeholder with a small icon and title; do not invent fake cover art.
- Use a subtle 1 px border only if needed to distinguish the card from the background.
- Default card corners: 3–5 px; avoid excessively rounded cards.

### Card states
**Default**
- Artwork only, or artwork with a small unobtrusive title treatment where needed.
- No heavy shadow.

**Hover**
- Slightly brighten artwork.
- Subtle 1–2 px lift is acceptable, but keep motion restrained.
- Optional thin accent border.
- Show secondary information only on hover, such as title, status, or quick action.
- Do not cover the artwork with a large opaque overlay.

**Selected**
- Subtle accent border or accent-tinted outline.
- Selection must be clear without a large glow.

**Keyboard focus**
- Visible focus ring using accent color.

### Library toolbar
Possible controls, only when relevant and implemented:
- All games / category selector.
- Platform or source filter.
- Sort dropdown.
- Filter button.
- Grid/list toggle.
- Search.
- Compact settings/view-options button.

Keep controls small and horizontally aligned. Toolbar should not become a second sidebar.

---

## 8. SAPIE Friends View

Friends should feel like a launcher social panel, not a CRM/contact-management table.

### Layout
- Page title: “Friends”.
- Small primary action such as “Add friend”.
- Search/filter control near the top.
- Friends grouped by status where possible:
  - Online
  - Away
  - Offline
- If groups are empty, show a small empty state instead of a large dashboard card.

### Friend row
- Avatar: 32–40 px square or circle.
- Username: 13–14 px, medium.
- Status: 11–12 px muted.
- Online indicator: small dot plus textual status.
- Optional context menu: compact icon button.
- Row height: around 48–56 px.
- Hover background: `--bg-panel-hover`.
- Do not use large profile cards for every friend.

### Friend actions
Use compact menus or buttons:
- View profile/details, if implemented.
- Invite to lobby, where valid.
- Remove friend.
- Block/unblock, where supported.

Destructive actions should be visually separated and use `--danger` only for the destructive label/icon. Confirm actions that are irreversible.

---

## 9. Friend Requests

- Use a compact list of incoming/outgoing requests.
- Display avatar, username, and request direction/status.
- Accept should be the primary action.
- Decline/remove should be secondary.
- Avoid oversized cards.
- Pending count may appear in the sidebar as a small badge.
- Empty state: concise message and a small illustration/icon at most.

---

## 10. Lobbies

Lobbies should use a library-like browsing layout, with clear information hierarchy.

### Lobby list
Each lobby row/card should show:
- Lobby name or host.
- Current player count / capacity, if available.
- Joinability or privacy state, if available.
- Relevant game or session context, if SAPIE has that data.
- Join action, only when joining is valid.

Prefer compact cards or rows. Do not use giant full-width dashboard tiles.

### Lobby states
- Joinable: normal text with subtle success accent.
- Full: muted state, clearly marked full.
- Private / invite-only: privacy indicator.
- Loading: skeleton placeholders that match the final layout.
- Error: inline error with retry action.

### Lobby detail
If a detail view is needed, use:
- Header with lobby title and status.
- Player list.
- Small action area for join/leave/invite.
- Minimal panel borders and separators.
- Do not invent game-specific information SAPIE does not know.

---

## 11. Empty, Loading, and Error States

### Empty states
- Small icon or restrained illustration.
- Short title.
- One sentence explaining the situation.
- One relevant action, if applicable.
- Avoid huge empty-state panels occupying most of the screen.

### Loading
- Prefer skeletons that mimic the actual layout.
- Keep shimmer subtle or use static skeleton blocks.
- Avoid excessive animated loading effects.
- Loading should not shift the layout significantly.

### Errors
- Use inline errors near the affected content.
- Use `--danger` sparingly.
- Explain the issue in plain language.
- Include Retry only when retrying is meaningful.
- Do not show raw stack traces or technical server errors to normal users.

---

## 12. Components and Shape Language

### Radius
- Small controls: 4–6 px.
- Cards: 4–6 px.
- Popovers/dropdowns: 6 px.
- Dialogs: 8 px maximum unless existing design system requires otherwise.
- Avoid overly rounded “mobile app” pills.

### Borders
- Use 1 px low-contrast borders.
- Prefer separators over nested boxes.
- Avoid wrapping every section in a card.
- Do not use bright borders as decoration.

### Shadows
- Very subtle shadows only for menus, dialogs, and elevated popovers.
- Main page should not look like a stack of floating cards.

### Icons
- Use one consistent icon family already in the project.
- Default icon size: 14–16 px in sidebar and toolbar.
- Avoid mixing icon styles.
- Icons should not replace labels when the action is not obvious.

---

## 13. Motion

Use motion to communicate state, not decorate.

- Hover transitions: 120–180 ms.
- Menu/dialog transitions: 140–200 ms.
- Use ease-out for entering and ease-in for leaving.
- Avoid bouncing, spring-heavy animation, continuous animated backgrounds, and excessive scale transforms.
- Respect reduced-motion settings.
- Grid/card movement should not cause layout jitter.

---

## 14. Search, Filters, and Sorting

- Search field should blend into the toolbar.
- Search placeholder should be short and descriptive.
- Filters should use compact dropdowns.
- Active filters should be visibly indicated, preferably with an accent tint rather than a bright badge.
- Provide a clear reset action when filters are active.
- Search should not force a full-screen modal unless the existing app architecture calls for it.

---

## 15. Responsive Behavior

### Wide desktop
- Full sidebar.
- Main toolbar.
- 5–7 cover columns depending on width.
- Comfortable spacing.

### Standard desktop
- Full sidebar.
- 4–5 cover columns.
- Compact toolbar.

### Narrow window
- Reduce grid columns to 2–3.
- Sidebar may collapse to icon-only if necessary.
- Keep search and primary actions accessible.
- Avoid horizontal overflow.
- Do not let controls overlap.

### Mobile
SAPIE is primarily a desktop-style LAN application. If mobile support is provided:
- Convert sidebar into a drawer or bottom navigation.
- Use 2-column grid for library-like layouts.
- Keep friend/lobby rows touch-friendly.
- Do not simply shrink the desktop layout.

---

## 16. Accessibility

- Maintain readable contrast, especially muted labels.
- Do not convey online/offline, errors, or selection by color alone.
- All interactive controls must be keyboard reachable.
- Use clear focus outlines.
- Buttons must have accessible names.
- Images should have meaningful alt text where appropriate; decorative artwork can use empty alt text if the game title is already available nearby.
- Support reduced motion.
- Keep text readable; do not use tiny labels for critical actions.

---

## 17. Do Not

- Do not build a generic SaaS dashboard.
- Do not use bright white backgrounds.
- Do not use large gradient hero banners.
- Do not use giant rounded cards.
- Do not make every section a bordered card.
- Do not overuse accent purple.
- Do not use neon/glow effects.
- Do not use oversized typography.
- Do not add storefront/purchase/install UI unless those features actually exist.
- Do not create fake data that looks like real users or real online activity in production.
- Do not clutter the sidebar with unimplemented pages.
- Do not put every action directly on screen; use compact context menus when appropriate.
- Do not sacrifice usability for visual similarity.

---

## 18. Implementation Guidance for Agents

When implementing UI:
1. Inspect the existing client stack and component conventions before introducing new libraries.
2. Reuse existing components, styles, routing, and state-management patterns where possible.
3. Keep design tokens centralized.
4. Keep the sidebar and top toolbar consistent between views.
5. Build shared primitives for navigation rows, icon buttons, compact dropdowns, status indicators, avatars, friend rows, lobby rows, and cover cards.
6. Ensure each page uses the same spacing and typography rules.
7. Use real API data and actual SAPIE capabilities. Do not fabricate unsupported features.
8. Include loading, empty, error, hover, active, selected, and keyboard-focus states.
9. Verify layout at desktop widths and at a narrow window width.
10. Avoid broad rewrites when a targeted UI change is sufficient.

The final result should feel like a **quiet, polished PC game launcher**: content-heavy, dark, compact, game-art-first, and easy to navigate.

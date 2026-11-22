# SAPIE identity surface copy

Copy for the surfaces named in `plan.md` 13.2.9 and M22-f. These are strings, not screenshots. The
mark is the provisional wordmark at `design/wordmark.svg`, pending G1 and not final art. Palette
values come from `design/tokens.css`. Voice follows `plan.md` 13.2.5 and `design/identity.md`.

Replace `[VERSION]`, `[OS]` and `[LISTEN]` with real values at build time. Do not fill a placeholder
with an estimate. If a value is unknown, leave it out. No em dash, no exclamation mark in a system
message, no invented player, stat or testimonial.

## Social preview

For the repository Open Graph image and link cards. Flat `--bg-app` field, the provisional wordmark
and the two lines below. No cover art, no player counts, no screenshots.

    Your games. Your server.
    Old Steam games, no Steam client.

## About screen

Shown in Settings / About on both launchers. Four short blocks, no scrolling claim and no version
guess.

    SAPIE [OS] [VERSION]

    Your games. Your server.

    Old Steam games, no Steam client. You own the game, the machine and the server.

    Non-commercial. Built for games you already own. The mark shown here is provisional and pending
    G1.

Links on this screen go to the docs, the source repository and the license. Each link has a real
destination. Do not show a Check for updates control unless the update channel is implemented.

## Installer splash

One line per state. Replace `[OS]` with Windows or Linux. Do not show a progress percentage unless a
real step count is available.

    Installing SAPIE for [OS]

    Preparing files. Do not close this window.

    Install complete. You can start SAPIE.

    Install failed. The original files were left in place. See the launcher log.

The error line names the state and one next step. It does not print a raw stack trace.

## Server startup banner

The server prints text, not ASCII art. It uses the wordmark name and the real listener values from
the flags in effect for that run. Values below are placeholders because they are configuration.

    SAPIE SERVER [VERSION]
    Your games. Your server.
    Game protocol  [GAME LISTEN]
    Admin API      [ADMIN LISTEN]
    Web UI         [UI LISTEN]

If TLS is enabled on the game listener, add one line: `Game listener TLS: on`. Do not print the admin
token in the banner. The generated admin token stays in the first-run log line only.

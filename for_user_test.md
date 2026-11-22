# What to test

This is a checklist. Do it in order. Write down anything that breaks and send it back.

Nothing here needs a test tool. You are the test.

Two things only you can check: the real game run (part 5) and the look (part 6). Everything else is a quick pass.

---

## Before you start

You need:

- The 0.5 download for your computer, unpacked into a folder.
- A real old Steam game you own. For example MW2 2009.
- A second computer, or a second Windows account, for the two-player test in part 5.
- Two SAPIE accounts. Make them while the server has registration turned on.

Words in `[brackets]` mean "put your own value here". For example `[server IP]` is something like `192.168.1.10:47584`.

---

## Part 1. Start the server

Do this on the computer that will be the server.

- [ ] Open a terminal in the unpacked folder.
- [ ] Run `./sapie-server` (Windows: `sapie-server.exe`).
- [ ] It starts. No error. It prints an admin token once. Save that token somewhere safe.
- [ ] Leave this window open. The server runs while it is open.
- [ ] On another computer on the same network, open `http://[server IP]:8081` in a browser. You should see the SAPIE sign-in page. If it does not load, the server address or firewall is wrong.

Known good defaults: game port `47584`, web page port `8081`. Do not change them for this test.

---

## Part 2. Make two accounts (web browser)

Do this once, while the server was started with registration on.

- [ ] Open `http://[server IP]:8081`.
- [ ] Register account A. Write down the name and password.
- [ ] Register account B. Write down the name and password.
- [ ] Log out.

If there is no Register button, the server was started without registration. Restart it with `--allow-register`, make the accounts, then restart it normally.

---

## Part 3. Launcher, one computer

Open the launcher (`sapie-launcher` or `sapie-launcher.exe`).

### Sign in

- [ ] First run shows the sign-in screen.
- [ ] Type `[server IP]`, account A name and password.
- [ ] Turn on "remember me".
- [ ] Press Sign in. It shows a pending state, then the library.
- [ ] Wrong password shows a clear "unauthorized" message, not a blank screen.
- [ ] A bad server address shows a clear "bad host" or "unreachable" message.

### Add a game

- [ ] In the library, press "Add a game" (or "Choose executable").
- [ ] Press "Choose executable". A real OS file picker opens. This is important. It must not be a fake list.
- [ ] Pick your game `.exe` or binary.
- [ ] The launcher shows the detected arch, era, appid and interface count before it writes anything.
- [ ] The appid is editable. If it is guessed, you can change it.
- [ ] Press "Add to library". It shows progress and finishes.
- [ ] Press "Choose executable" again, then cancel the file picker. Nothing bad happens. No error. You return to the "Choose an executable" screen.

If you already have this game in the library, the launcher should say "Already in your library" and offer to open that record.

### Library and game page

- [ ] The game appears as a tile with a name and an install state.
- [ ] Search finds it. Filter by installed and not installed works. Sort by name works.
- [ ] Open the game page. It shows path, arch, era, appid, interface list, library state and backup state.
- [ ] Play, Repair and Restore original buttons do something visible. A slow action shows progress.
- [ ] A destructive action such as Restore asks for confirmation first.
- [ ] Open the folder action opens the real folder on your computer.

### Friends, lobbies, settings, logs

- [ ] Friends screen shows an empty or loading state, then a real list. No made-up players.
- [ ] Send a friend request to account B from the web UI. Accept it in the web UI. The launcher friend list updates.
- [ ] Lobbies screen shows real lobbies or an honest empty state.
- [ ] Settings shows the account, server, update channel, launcher version and log location.
- [ ] Sign out clears the local token. Signing back in works.
- [ ] Logs copy to clipboard. No password appears in the log. Search the copied text for your password. It must not be there.

### Keyboard only

- [ ] Unplug the mouse. Tab moves through controls in the same order they appear on screen.
- [ ] Enter activates the focused control.
- [ ] Escape closes dialogs and returns focus to the button that opened them.

---

## Part 4. Web UI, second computer

Open `http://[server IP]:8081` in a browser.

- [ ] Log in as account B.
- [ ] Friends: see account A as a friend. Remove and block work, and unblock restores the friend.
- [ ] Invite link: copy a `sapie://join/...` link from the web UI.
- [ ] On the first computer, with the launcher already running, paste or open that same invite link. The launcher reacts to it. It must not open a second launcher window.
- [ ] Search, filters and empty states show honest messages or real data.
- [ ] Log out works.

---

## Part 5. The real game run (G0)

This is the one that matters most. Only you can do it.

- [ ] Start the server.
- [ ] On computer 1, sign in as account A and launch the game from the launcher. The game boots.
- [ ] On computer 2, sign in as account B and launch the same game. The game boots.
- [ ] The two clients see each other as friends in game.
- [ ] One client creates or joins a lobby. The other joins it.
- [ ] Both players are in the same game session. You can see each other move or play.
- [ ] Check the server log. It shows two real client connections and the traffic between them.

If the two players are in different sessions, or one cannot join, write down exactly what each screen showed and what the server log said.

This test is not done until a real game ran with two real clients. A headless test does not count.

---

## Part 6. The look (G1)

This is a yes or no check. Do not overthink it.

- [ ] Open Sign in, Library, Game, Friends and Settings.
- [ ] The mark and wordmark look like SAPIE, not GOG or Steam.
- [ ] Colours are quiet violet on dark. No glass panels. No ember orange as the main accent.
- [ ] Text is readable at normal size. Small text is readable too.
- [ ] Cover images are either yours or an honest letter tile. No invented game art.
- [ ] Placeholders look like placeholders. No fake players, stats or reviews.
- [ ] No button that does nothing. No "coming soon" on a button that looks clickable.
- [ ] No em dash in the text.

If any of these is wrong, say which screen and what you saw.

---

## What to send back

For each failed box, tell me:

1. What computer and OS (Windows 10, Windows 11, Ubuntu, and so on).
2. What screen you were on.
3. What you expected.
4. What happened instead.
5. A screenshot if you can, with passwords and tokens hidden.

If everything passes, tell me which parts you did and say "parts 1 to 6 pass". I will write the evidence file and sign the gates.

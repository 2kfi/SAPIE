# Third-party notice — Steamless

This directory bundles the **Steamless CLI, version 3.1.0.5**, by **atom0s**.

- Upstream: https://github.com/atom0s/Steamless/releases/tag/v3.1.0.5
- License: [CC BY-NC-ND 4.0](https://creativecommons.org/licenses/by-nc-nd/4.0/)
- Copyright: © 2015–2024 atom0s

## What is bundled

`Steamless.CLI.exe`, its `.config`, `infos.txt`, and the complete `Plugins/` folder,
copied verbatim from the upstream CLI release. The binaries are **unmodified**: they
are shipped byte-for-byte as released. No C# source was ported into SAPIE, and no
other third-party unpacker DLLs (for example ALI213 or CreamAPI) are used.

The launcher invokes the sidecar as:

```
Steamless.CLI.exe <path-to-game-exe>
```

This follows the CLI's own help text (`Steamless.CLI.exe [options] [file]`). On success
Steamless writes `<name>.unpacked.exe` next to the input file; if the input is not
SteamStub-packed, it reports that no unpacker applied and the launcher keeps the
original executable.

## Non-commercial commitment

SAPIE is a non-commercial project — "for gamers by gamers". The Steamless binaries are
included under the CC BY-NC-ND 4.0 non-commercial terms and are not sold, sublicensed,
or distributed with any commercial product. If SAPIE's distribution ever becomes
commercial, this sidecar must be removed before that change ships.

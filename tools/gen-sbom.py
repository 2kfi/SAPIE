#!/usr/bin/env python3
"""Write one SPDX JSON SBOM per SAPIE release artifact family."""

import argparse
import datetime
import json
import re
import tomllib
from pathlib import Path

FAMILIES = ("launcher-windows", "launcher-linux", "server", "dll", "web", "container")

DECLARED = {
    "launcher-windows": [
        ("Qt", "6.8", "LGPL-3.0-only", "dynamic, relink instructions in docs/THIRD_PARTY_NOTICES.md"),
        ("Steamless", "vendored", "CC-BY-NC-ND-4.0", "Windows sidecar, unmodified"),
        ("Manrope", "vendored", "OFL-1.1", "UI font"),
        ("IBM Plex Mono", "vendored", "OFL-1.1", "machine-value font"),
    ],
    "launcher-linux": [
        ("Qt", "6.8", "LGPL-3.0-only", "dynamic, relink instructions in docs/THIRD_PARTY_NOTICES.md"),
        ("Manrope", "vendored", "OFL-1.1", "UI font"),
        ("IBM Plex Mono", "vendored", "OFL-1.1", "machine-value font"),
    ],
    "server": [("Go", "1.26", "BSD-3-Clause", "toolchain and standard library")],
    "dll": [
        ("Goldberg steam_api fork", "vendored", "LGPL-3.0-only", "game-side Steam API emulation"),
        ("protobuf-lite", "v36.1", "BSD-3-Clause", "vendored build input"),
    ],
    "web": [
        ("GSAP", "3.15.1", "NOASSERTION", "GSAP standard license, web UI only"),
        ("Manrope", "vendored", "OFL-1.1", "UI font"),
        ("IBM Plex Mono", "vendored", "OFL-1.1", "machine-value font"),
    ],
    "container": [
        ("Alpine Linux", "3.23", "NOASSERTION", "base image"),
        ("ca-certificates", "vendored", "NOASSERTION", "base image package"),
    ],
}


def slug(text):
    return re.sub(r"[^A-Za-z0-9.-]", "-", text).strip("-") or "unknown"


def cargo_components(repo):
    path = repo / "client/src-tauri/Cargo.lock"
    if not path.is_file():
        return []
    data = tomllib.loads(path.read_text())
    return [(p["name"], p["version"], "NOASSERTION", "Rust crate, see Cargo.lock") for p in data.get("package", [])]


def go_components(repo):
    path = repo / "server/go.mod"
    if not path.is_file():
        return []
    result = []
    for line in path.read_text().splitlines():
        match = re.match(r"\s+(\S+)\s+(v\S+)", line)
        if match:
            result.append((match.group(1), match.group(2), "NOASSERTION", "Go module, see go.mod"))
    return result


def npm_components(repo):
    path = repo / "web/package-lock.json"
    if not path.is_file():
        return []
    data = json.loads(path.read_text())
    result = []
    for key, info in data.get("packages", {}).items():
        if not key.startswith("node_modules/"):
            continue
        name = key.rsplit("node_modules/", 1)[-1]
        result.append((name, info.get("version", "unknown"), info.get("license", "NOASSERTION"), "npm package, build-time unless listed as a runtime dependency"))
    return result


def package(name, version, license_name, comment):
    identifier = f"SPDXRef-Package-{slug(name)}-{slug(version)}"
    return {
        "SPDXID": identifier,
        "name": name,
        "versionInfo": version,
        "licenseConcluded": license_name,
        "licenseDeclared": license_name,
        "downloadLocation": "NOASSERTION",
        "filesAnalyzed": False,
        "copyrightText": "NOASSERTION",
        "comment": comment,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--repo", default=str(Path(__file__).resolve().parent.parent))
    args = parser.parse_args()

    repo = Path(args.repo)
    output = Path(args.output_dir)
    output.mkdir(parents=True, exist_ok=True)
    rust = cargo_components(repo)
    go = go_components(repo)
    npm = npm_components(repo)
    created = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

    for family in FAMILIES:
        components = list(DECLARED[family])
        if family.startswith("launcher-"):
            components += rust
        if family == "server":
            components += go
        if family == "web":
            components += npm
        if family == "container":
            components += go + npm

        packages = []
        seen = set()
        for name, version, license_name, comment in components:
            identifier = f"SPDXRef-Package-{slug(name)}-{slug(version)}"
            if identifier in seen:
                continue
            seen.add(identifier)
            packages.append(package(name, version, license_name, comment))

        doc = {
            "spdxVersion": "SPDX-2.3",
            "dataLicense": "CC0-1.0",
            "SPDXID": "SPDXRef-DOCUMENT",
            "name": f"SAPIE-{args.tag}-{family}",
            "documentNamespace": f"https://github.com/2kfi/SAPIE/releases/tag/{args.tag}#{family}",
            "creationInfo": {"created": created, "creators": ["Tool: tools/gen-sbom.py"]},
            "packages": packages,
            "relationships": [
                {"spdxElementId": "SPDXRef-DOCUMENT", "relatedSpdxElement": p["SPDXID"], "relationshipType": "DESCRIBES"}
                for p in packages
            ],
        }
        target = output / f"SBOM-{args.tag}-{family}.spdx.json"
        target.write_text(json.dumps(doc, indent=2, sort_keys=True) + "\n")
        print(f"wrote {target} ({len(packages)} packages)")


if __name__ == "__main__":
    main()

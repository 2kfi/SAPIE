#!/usr/bin/env python3
"""Generate and verify signed SAPIE update manifests."""

import argparse
import base64
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

SCHEMA = "sapie-update/v1"
CHANNELS = ("stable", "beta")
RELEASE_BASE = "https://github.com/2kfi/SAPIE/releases/download"


def die(message):
    print(f"error: {message}", file=sys.stderr)
    raise SystemExit(1)


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def normalized_version(tag):
    return tag if tag.count(".") >= 2 else f"{tag}.0"


def canonical(payload):
    return json.dumps(payload, sort_keys=True, separators=(",", ":"), ensure_ascii=True).encode()


def private_key_path(spec):
    if not spec:
        die("no signing key: pass --private-key or set SAPIE_UPDATE_SIGNING_KEY")
    if "-----BEGIN" not in spec:
        path = Path(spec)
        if not path.is_file():
            die(f"signing key not found: {spec}")
        return path, None
    handle = tempfile.NamedTemporaryFile(mode="w", suffix=".pem", delete=False)
    os.chmod(handle.name, 0o600)
    handle.write(spec.strip() + "\n")
    handle.close()
    return Path(handle.name), handle.name


def sign(payload_file, key_file, sig_file):
    subprocess.run(
        ["openssl", "pkeyutl", "-sign", "-rawin", "-inkey", str(key_file),
         "-in", str(payload_file), "-out", str(sig_file)],
        check=True,
    )


def verify_signature(payload_file, public_key, sig_file):
    result = subprocess.run(
        ["openssl", "pkeyutl", "-verify", "-pubin", "-rawin", "-inkey", str(public_key),
         "-in", str(payload_file), "-sigfile", str(sig_file)],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    if result.returncode != 0:
        die("signature verification failed: manifest is tampered or the public key is wrong")


def generate(args):
    if args.channel not in CHANNELS:
        die(f"unknown channel: {args.channel}")
    artifacts = Path(args.artifacts)
    if not artifacts.is_dir():
        die(f"artifacts directory not found: {artifacts}")
    public_key = Path(args.public_key)
    if not public_key.is_file():
        die(f"public key not found: {public_key}")

    key_file, temp_path = private_key_path(args.private_key or os.environ.get("SAPIE_UPDATE_SIGNING_KEY"))
    try:
        files = []
        for path in sorted(artifacts.iterdir()):
            name = path.name
            if path.is_file() and re.fullmatch(rf"SAPIE-{re.escape(args.tag)}-.+", name):
                files.append({
                    "name": name,
                    "os": "windows" if "-windows-" in name else "linux",
                    "arch": "arm64" if "arm64" in name else "x64",
                    "sha256": sha256(path),
                    "size": path.stat().st_size,
                    "url": f"{RELEASE_BASE}/{args.tag}/{name}",
                })
        if not files:
            die(f"no SAPIE-{args.tag}-* artifacts in {artifacts}")

        payload = {
            "schema": SCHEMA,
            "channel": args.channel,
            "version": normalized_version(args.tag),
            "tag": args.tag,
            "artifacts": files,
            "previous_version": args.previous_version or None,
            "rollback": "Manual: download the previous signed release and reinstall it. See docs/BUILD.md.",
        }
        if args.checksums:
            payload["checksums_sha256"] = sha256(Path(args.checksums))

        body = canonical(payload)
        output = Path(args.output)
        sig_file = Path(str(output) + ".sig")
        output.write_bytes(body)
        sign(output, key_file, sig_file)
        sig_file.write_bytes(base64.b64encode(sig_file.read_bytes()))
        print(f"wrote {output} and {sig_file} ({args.channel}, {payload['version']})")
    finally:
        if temp_path:
            os.unlink(temp_path)


def verify(args):
    manifest = Path(args.manifest)
    if not manifest.is_file():
        die(f"manifest not found: {manifest}")
    sig_file = Path(str(manifest) + ".sig")
    if not sig_file.is_file():
        die(f"signature not found: {sig_file}")
    public_key = Path(args.public_key)
    if not public_key.is_file():
        die(f"public key not found: {public_key}")

    body = manifest.read_bytes()
    try:
        signature = base64.b64decode(sig_file.read_bytes(), validate=True)
    except ValueError:
        die("invalid base64 in manifest signature")
    with tempfile.NamedTemporaryFile(suffix=".sig", delete=False) as temp:
        raw_sig = Path(temp.name)
    raw_sig.write_bytes(signature)
    try:
        verify_signature(manifest, public_key, raw_sig)
    finally:
        raw_sig.unlink(missing_ok=True)

    payload = json.loads(body)
    if payload.get("schema") != SCHEMA:
        die(f"unsupported manifest schema: {payload.get('schema')}")
    if payload.get("channel") not in CHANNELS:
        die(f"unsupported manifest channel: {payload.get('channel')}")
    if args.channel and payload["channel"] != args.channel:
        die(f"channel mismatch: expected {args.channel}, got {payload['channel']}")
    if args.version and payload.get("version") != args.version:
        die(f"version mismatch: expected {args.version}, got {payload.get('version')}")
    if not payload.get("artifacts"):
        die("manifest has no artifacts")
    for artifact in payload["artifacts"]:
        if not re.fullmatch(r"[0-9a-f]{64}", str(artifact.get("sha256", ""))):
            die(f"bad sha256 for {artifact.get('name')}")
    print(f"verified {manifest}: channel={payload['channel']} version={payload['version']}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)

    gen = sub.add_parser("generate")
    gen.add_argument("--tag", required=True)
    gen.add_argument("--artifacts", required=True)
    gen.add_argument("--output", required=True)
    gen.add_argument("--public-key", required=True)
    gen.add_argument("--private-key")
    gen.add_argument("--channel", default="stable", choices=CHANNELS)
    gen.add_argument("--previous-version")
    gen.add_argument("--checksums")
    gen.set_defaults(func=generate)

    ver = sub.add_parser("verify")
    ver.add_argument("--manifest", required=True)
    ver.add_argument("--public-key", required=True)
    ver.add_argument("--channel", choices=CHANNELS)
    ver.add_argument("--version")
    ver.set_defaults(func=verify)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()

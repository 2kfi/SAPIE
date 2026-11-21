#!/usr/bin/env python3
"""Fail on high-confidence secrets in tracked source files."""

import re
import subprocess
import sys

PATTERNS = [
    ("private key", re.compile(r"-----BEGIN (?:[A-Z ]+ )?PRIVATE KEY-----")),
    ("AWS access key", re.compile(r"\bAKIA[0-9A-Z]{16}\b")),
    ("GitHub token", re.compile(r"\bgh[pousr]_[A-Za-z0-9]{36,}\b")),
    ("Slack token", re.compile(r"\bxox[abprs]-[A-Za-z0-9-]{10,}\b")),
]
SKIP_SUFFIXES = (".dll", ".exe", ".so", ".a", ".o", ".png", ".jpg", ".woff2", ".ttf", ".pdf")
# Vendored Steam headers document key formats with sample PEM labels.
SKIP_PATHS = ("client/sdk_includes/",)


def main():
    tracked = subprocess.run(["git", "ls-files", "-z"], check=True, stdout=subprocess.PIPE).stdout
    findings = []
    for raw in tracked.split(b"\0"):
        if not raw:
            continue
        name = raw.decode("utf-8", "replace")
        if name.startswith(SKIP_PATHS) or name.endswith(SKIP_SUFFIXES):
            continue
        try:
            with open(name, "rb") as stream:
                data = stream.read()
        except OSError as exc:
            print(f"error: cannot read {name}: {exc}", file=sys.stderr)
            sys.exit(1)
        if b"\0" in data:
            continue
        for number, line in enumerate(data.decode("utf-8", "replace").splitlines(), 1):
            for label, pattern in PATTERNS:
                if pattern.search(line):
                    findings.append(f"{name}:{number}: {label}")
    if findings:
        print("secret scan failed:", file=sys.stderr)
        for finding in findings:
            print(f"  {finding}", file=sys.stderr)
        sys.exit(1)
    print("secret scan: no high-confidence secrets in tracked files")


if __name__ == "__main__":
    main()

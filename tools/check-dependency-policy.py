#!/usr/bin/env python3
"""Keep the npm advisory allowlist in CI and docs identical."""

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
WORKFLOW = ROOT / ".github/workflows/ci.yml"
POLICY = ROOT / "docs/DEPENDENCY_POLICY.md"


def workflow_ids():
    lines = WORKFLOW.read_text().splitlines()
    for index, line in enumerate(lines):
        if "ACCEPTED_NPM_ADVISORIES:" not in line:
            continue
        ids = []
        for following in lines[index + 1:]:
            if not following.startswith(" "):
                break
            ids += re.findall(r"GHSA-[0-9a-z-]+", following)
        return set(ids)
    sys.exit("error: ACCEPTED_NPM_ADVISORIES not found in .github/workflows/ci.yml")


def policy_ids():
    ids = set()
    for line in POLICY.read_text().splitlines():
        match = re.match(r"\|\s*(GHSA-[0-9a-z-]+)\s*\|\s*([^|]+?)\s*\|", line)
        if match:
            if not match.group(2):
                sys.exit(f"error: empty reason for {match.group(1)}")
            ids.add(match.group(1))
    return ids


def main():
    workflow, policy = workflow_ids(), policy_ids()
    if workflow == policy:
        print(f"dependency policy: {len(policy)} accepted advisories match")
        return
    print("dependency policy mismatch", file=sys.stderr)
    for label, only in (("CI only", workflow - policy), ("docs only", policy - workflow)):
        if only:
            print(f"  {label}: {', '.join(sorted(only))}", file=sys.stderr)
    sys.exit(1)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Deterministic monogram tile for a SAPIE appid. Stdlib only.

Provisional until G1. The tile is geometry, not cover art: no font, no network.
"""

import hashlib
import sys
import xml.etree.ElementTree as ET
from xml.sax.saxutils import escape

VIOLET = "#C5A4FF"
BACKGROUNDS = ("#17191D", "#1D2025", "#292D34", "#342A46")


def tile(appid):
    digest = hashlib.sha256(appid.encode("utf-8")).digest()
    bg_index = digest[0] % len(BACKGROUNDS)
    bg = BACKGROUNDS[bg_index]
    shade = BACKGROUNDS[(bg_index + 1 + digest[3] % 3) % len(BACKGROUNDS)]
    motif = digest[1] % 4
    turn = (digest[2] % 4) * 90

    if motif == 0:
        art = ('<circle cx="32" cy="30" r="16" fill="%s"/>'
               '<rect x="42" y="42" width="12" height="12" rx="2" fill="%s"/>') % (VIOLET, shade)
    elif motif == 1:
        art = ('<rect x="14" y="14" width="12" height="36" rx="2" fill="%s"/>'
               '<circle cx="40" cy="32" r="12" fill="%s"/>') % (VIOLET, VIOLET)
    elif motif == 2:
        art = ('<circle cx="32" cy="32" r="17" fill="none" stroke="%s" stroke-width="6"/>'
               '<circle cx="32" cy="32" r="5" fill="%s"/>') % (VIOLET, shade)
    else:
        art = ('<path d="M22 16 L46 32 L22 48 Z" fill="%s"/>'
               '<circle cx="44" cy="48" r="5" fill="%s"/>') % (VIOLET, shade)

    return (
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width="64" height="64" role="img">\n'
        '  <title>Monogram for appid %s. Provisional, pending G1.</title>\n'
        '  <rect x="0" y="0" width="64" height="64" rx="8" fill="%s"/>\n'
        '  <g transform="rotate(%d 32 32)">%s</g>\n'
        '</svg>\n'
    ) % (escape(appid), bg, turn, art)


def self_test():
    first = tile("440010")
    assert first == tile("440010"), "same appid must produce the same tile"
    assert first != tile("440020"), "different appids must produce different tiles"
    ET.fromstring(first)
    print("self-test ok")


if __name__ == "__main__":
    if len(sys.argv) == 2 and sys.argv[1] == "--self-test":
        self_test()
    elif len(sys.argv) == 2:
        sys.stdout.write(tile(sys.argv[1]))
    else:
        sys.exit("usage: gen-monogram.py APPID | --self-test")

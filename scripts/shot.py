#!/usr/bin/env python3
"""Turn `tuna --preview` into the SVG the README shows.

Run through `just shot`. The point is that the picture cannot drift from the
program: it is the real ANSI output of the real binary, parsed and drawn,
rather than a terminal screenshot someone took once and never refreshed.

A PNG would need a terminal, a font and a screen. An SVG needs none of the
three, stays a few kilobytes, and reads sharp at any zoom.
"""

import os
import re
import shutil
import subprocess
import pathlib
import sys
import tempfile

# The palette. Close to what a dark terminal shows, and chosen to stay legible
# on the light GitHub theme too, since the panel carries its own background.
FG = "#c9d1d9"
BG = "#0d1117"
COLOURS = {
    "0": None,  # reset
    "1": ("#f0f6fc", "bold"),
    "2": ("#6e7681", None),
    "4": (None, "underline"),
    "24": (None, "-underline"),
    "31": ("#f85149", None),
    "32": ("#3fb950", None),
    "33": ("#d29922", None),
    "36": ("#39c5cf", None),
}

CELL_W, LINE_H, PAD = 8.4, 21.0, 22.0
TOP = 46.0  # room for the window bar

SGR = re.compile(r"\x1b\[([0-9;]*)m")


def runs(line):
    """Split one ANSI line into (text, colour, bold, underline) runs."""
    out, pos = [], 0
    colour, bold, under = None, False, False
    for m in SGR.finditer(line):
        if m.start() > pos:
            out.append((line[pos : m.start()], colour, bold, under))
        for code in (m.group(1) or "0").split(";"):
            if code == "0":
                colour, bold, under = None, False, False
            elif code in COLOURS and COLOURS[code]:
                c, flag = COLOURS[code]
                if c:
                    colour = c
                if flag == "bold":
                    bold = True
                elif flag == "underline":
                    under = True
                elif flag == "-underline":
                    under = False
        pos = m.end()
    if pos < len(line):
        out.append((line[pos:], colour, bold, under))
    return out


def esc(s):
    return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def main():
    binary, out_path, width = sys.argv[1], sys.argv[2], int(sys.argv[3])
    root = pathlib.Path(__file__).resolve().parent.parent

    # The example config, never the real one. This picture is published, and
    # a real destinations.toml holds internal addresses and non-standard ssh
    # ports: pointing the binary at a throwaway XDG root is what makes it
    # impossible to leak one by running this on the wrong machine.
    with tempfile.TemporaryDirectory() as tmp:
        cfg = pathlib.Path(tmp) / "config" / "tuna"
        cfg.mkdir(parents=True)
        shutil.copy(root / "destinations.example.toml", cfg / "destinations.toml")

        # CLICOLOR_FORCE keeps the escape codes alive through the pipe.
        proc = subprocess.run(
            [binary, "--preview", str(width)],
            capture_output=True,
            text=True,
            env={
                **{k: v for k, v in os.environ.items() if k != "NO_COLOR"},
                "CLICOLOR_FORCE": "1",
                "XDG_CONFIG_HOME": f"{tmp}/config",
                "XDG_STATE_HOME": f"{tmp}/state",
            },
            check=True,
        )

    # --preview prints several states under dim "── title ──" rules. The hero
    # is the first one: the list as it opens.
    frames = re.split(r"\x1b\[2m── .*?\x1b\[0m", proc.stdout)
    lines = frames[1].split("\n")[1:]
    # Trailing blanks are the frame's own bottom margin, and PAD already is
    # one: keeping both leaves the panel looking bottom-heavy.
    while lines and not lines[-1].strip():
        lines.pop()

    w = width * CELL_W + PAD * 2
    h = TOP + (len(lines) - 1) * LINE_H + PAD
    svg = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{w:.0f}" height="{h:.0f}" '
        f'viewBox="0 0 {w:.0f} {h:.0f}" role="img" aria-label="tuna, la liste des destinations">',
        "<title>tuna — choisir sa destination au lieu de la mémoriser</title>",
        f'<rect width="{w:.0f}" height="{h:.0f}" rx="10" fill="{BG}"/>',
    ]
    for i, c in enumerate(("#ff5f57", "#febc2e", "#28c840")):
        svg.append(f'<circle cx="{PAD + 4 + i * 18:.0f}" cy="24" r="6" fill="{c}"/>')
    svg.append(
        f'<text x="{w / 2:.0f}" y="29" fill="#6e7681" text-anchor="middle" '
        f'font-family="ui-monospace,SFMono-Regular,Menlo,Consolas,monospace" '
        f'font-size="12">tuna</text>'
    )
    svg.append(f'<path d="M0 40 H{w:.0f}" stroke="#21262d" stroke-width="1"/>')

    # xml:space keeps the leading spaces that carry the whole layout.
    svg.append(
        f'<g font-family="ui-monospace,SFMono-Regular,Menlo,Consolas,monospace" '
        f'font-size="14" xml:space="preserve">'
    )
    for row, line in enumerate(lines):
        y = TOP + row * LINE_H
        col = 0
        for text, colour, bold, under in runs(line):
            if text.strip():
                attrs = f'x="{PAD + col * CELL_W:.1f}" y="{y:.1f}" fill="{colour or FG}"'
                if bold:
                    attrs += ' font-weight="600"'
                if under:
                    attrs += ' text-decoration="underline"'
                svg.append(f"<text {attrs}>{esc(text)}</text>")
            col += len(text)
    svg.append("</g></svg>")

    with open(out_path, "w") as f:
        f.write("\n".join(svg) + "\n")
    print(f"{out_path}: {len(lines)} lignes, {w:.0f}×{h:.0f}")


if __name__ == "__main__":
    main()

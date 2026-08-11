#!/usr/bin/env python3
"""Draw the pictures the README and GitHub show, from the real program.

Run through `just shot`. The point is that they cannot drift: each one is the
actual ANSI output of the actual binary, parsed and redrawn, rather than a
screenshot someone took once and never refreshed.

An SVG needs no terminal, no font and no screen, stays a couple of kilobytes,
and reads sharp at any zoom. The social card is rasterised on top of that only
because GitHub will not take an SVG for it.
"""

import os
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

MONO = "ui-monospace,SFMono-Regular,Menlo,Consolas,monospace"
SANS = "-apple-system,BlinkMacSystemFont,Segoe UI,Helvetica,Arial,sans-serif"

# Close to what a dark terminal shows, and legible on GitHub's light theme too,
# since the panel carries its own background either way.
FG, BG = "#c9d1d9", "#0d1117"
COLOURS = {
    "1": ("#f0f6fc", "bold"),
    "2": ("#6e7681", None),
    "4": (None, "underline"),
    "24": (None, "-underline"),
    "31": ("#f85149", None),
    "32": ("#3fb950", None),
    "33": ("#d29922", None),
    "36": ("#39c5cf", None),
}

SGR = re.compile(r"\x1b\[([0-9;]*)m")
RULE = re.compile(r"\x1b\[2m── .*?\x1b\[0m")


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
            elif code in COLOURS:
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


def panel(lines, ox, oy, cols, size=14):
    """The terminal panel: background, window bar, then the text as drawn."""
    cell, lh, pad, top = size * 0.6, size * 1.5, 22.0, 46.0
    w = cols * cell + pad * 2
    h = top + (len(lines) - 1) * lh + pad

    out = [f'<g transform="translate({ox:.0f},{oy:.0f})">']
    out.append(f'<rect width="{w:.0f}" height="{h:.0f}" rx="10" fill="{BG}"/>')
    for i, c in enumerate(("#ff5f57", "#febc2e", "#28c840")):
        out.append(f'<circle cx="{pad + 4 + i * 18:.0f}" cy="24" r="6" fill="{c}"/>')
    out.append(
        f'<text x="{w / 2:.0f}" y="29" fill="#6e7681" text-anchor="middle"'
        f' font-family="{MONO}" font-size="12">tuna</text>'
    )
    out.append(f'<path d="M0 40 H{w:.0f}" stroke="#21262d" stroke-width="1"/>')

    # xml:space keeps the leading spaces that carry the whole layout.
    out.append(f'<g font-family="{MONO}" font-size="{size}" xml:space="preserve">')
    for row, line in enumerate(lines):
        y, col = top + row * lh, 0
        for text, colour, bold, under in runs(line):
            if text.strip():
                a = f'x="{pad + col * cell:.1f}" y="{y:.1f}" fill="{colour or FG}"'
                if bold:
                    a += ' font-weight="600"'
                if under:
                    a += ' text-decoration="underline"'
                out.append(f"<text {a}>{esc(text)}</text>")
            col += len(text)
    out.append("</g></g>")
    return out, w, h


def frames_of(binary, cols):
    """The picker's states, as the binary itself draws them."""
    root = pathlib.Path(__file__).resolve().parent.parent

    # The example config, never the real one. These pictures are published and
    # a real destinations.toml holds internal addresses and non-standard ssh
    # ports: a throwaway XDG root is what makes it impossible to leak one by
    # running this on the wrong machine.
    with tempfile.TemporaryDirectory() as tmp:
        cfg = pathlib.Path(tmp) / "config" / "tuna"
        cfg.mkdir(parents=True)
        shutil.copy(root / "destinations.example.toml", cfg / "destinations.toml")
        env = {k: v for k, v in os.environ.items() if k != "NO_COLOR"}
        proc = subprocess.run(
            [binary, "--preview", str(cols)],
            capture_output=True,
            text=True,
            check=True,
            env={
                # CLICOLOR_FORCE keeps the escape codes alive through the pipe.
                # NO_COLOR is dropped above, because it would win over it.
                **env,
                "CLICOLOR_FORCE": "1",
                "XDG_CONFIG_HOME": f"{tmp}/config",
                "XDG_STATE_HOME": f"{tmp}/state",
            },
        )

    frames = []
    for frame in RULE.split(proc.stdout)[1:]:
        lines = frame.split("\n")[1:]
        # Trailing blanks are the frame's own bottom margin, and the panel
        # already has one: keeping both leaves it looking bottom-heavy.
        while lines and not lines[-1].strip():
            lines.pop()
        frames.append(lines)
    return frames


def write(path, body):
    path.write_text(body + "\n")
    print(f"  {path.name}  {len(body) // 1024 or 1} ko")


def main():
    binary, cols = sys.argv[1], int(sys.argv[2])
    assets = pathlib.Path(__file__).resolve().parent.parent / "docs" / "assets"
    lines = frames_of(binary, cols)[0]

    body, w, h = panel(lines, 0, 0, cols)
    write(
        assets / "picker.svg",
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{w:.0f}" height="{h:.0f}"'
        f' viewBox="0 0 {w:.0f} {h:.0f}" role="img"'
        f' aria-label="La liste des destinations de tuna">\n'
        f"<title>tuna — choisir sa destination au lieu de la mémoriser</title>\n"
        + "\n".join(body)
        + "\n</svg>",
    )

    # The social card, which is what GitHub shows when the link is shared.
    # 1280 by 640 is the size it renders at; anything else gets resampled.
    cw, ch = 1280, 640
    card = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{cw}" height="{ch}"'
        f' viewBox="0 0 {cw} {ch}" role="img" aria-label="tuna">',
        f'<rect width="{cw}" height="{ch}" fill="#010409"/>',
        f'<text x="{cw // 2}" y="152" fill="#f0f6fc" text-anchor="middle"'
        f' font-family="{MONO}" font-size="78" font-weight="700">tuna</text>',
        f'<text x="{cw // 2}" y="208" fill="#39c5cf" text-anchor="middle"'
        f' font-family="{SANS}" font-size="27">choisir sa destination au lieu'
        f" de la mémoriser</text>",
        f'<text x="{cw // 2}" y="248" fill="#6e7681" text-anchor="middle"'
        f' font-family="{SANS}" font-size="19">un tunnel SSH d’admin qui survit'
        f" à un wifi qui saute</text>",
    ]
    card += panel(lines, (cw - w) / 2, 310, cols)[0] + ["</svg>"]
    write(assets / "social-card.svg", "\n".join(card))

    # GitHub's social preview takes PNG, JPG or GIF, never SVG. Rasterised when
    # the tool is around and skipped with a word when it is not: the card is
    # uploaded by hand anyway, so a missing rasteriser is not a failure.
    png = assets / "social-card.png"
    if shutil.which("rsvg-convert"):
        subprocess.run(
            ["rsvg-convert", "-w", str(cw), "-h", str(ch),
             "-o", str(png), str(assets / "social-card.svg")],
            check=True,
        )
        print(f"  {png.name}  {png.stat().st_size // 1024} ko")
    else:
        print("  (rsvg-convert absent : social-card.png non régénérée)")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Regenerate PdmView's Windows amd64 icon resource from assets/PdmView.png."""

import shutil
import struct
import subprocess
import tempfile
from pathlib import Path

SIZES = (16, 24, 32, 48, 64, 128, 256)
ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / "assets" / "PdmView.png"
WORK = ROOT / ".out" / "windowsicon"
OUTPUT = ROOT / "cmd" / "pdmview" / "pdmview_windows_amd64.syso"


def command(*args):
    subprocess.run(args, check=True)


def main():
    png = SOURCE.read_bytes()
    if png[:8] != b"\x89PNG\r\n\x1a\n" or png[12:16] != b"IHDR":
        raise SystemExit(f"{SOURCE}: expected a PNG")
    width, height = struct.unpack_from(">II", png, 16)
    if width != height:
        raise SystemExit(f"{SOURCE}: icon must be square, got {width}x{height}")

    sips = shutil.which("sips")
    if not sips:
        raise SystemExit("sips is required to regenerate Windows icons")
    WORK.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="generate.", dir=WORK))
    images = []
    for size in SIZES:
        image = work / f"PdmView-{size}.png"
        command(sips, "-z", str(size), str(size), str(SOURCE), "--out", str(image))
        images.append(image.read_bytes())

    offset = 6 + 16 * len(images)
    ico = [struct.pack("<HHH", 0, 1, len(images))]
    for size, image in zip(SIZES, images):
        dimension = 0 if size == 256 else size
        ico.append(struct.pack("<BBBBHHII", dimension, dimension, 0, 0, 1, 32,
                               len(image), offset))
        offset += len(image)
    ico_path = work / "PdmView.ico"
    ico_path.write_bytes(b"".join(ico + images))

    resource_object = work / OUTPUT.name
    command("go", "run",
            "github.com/akavel/rsrc@v0.10.2",
            "-ico", str(ico_path), "-arch", "amd64", "-o", str(resource_object))
    resource_object.replace(OUTPUT)
    print(f"wrote {OUTPUT} from {SOURCE} via {ico_path}")


if __name__ == "__main__":
    main()

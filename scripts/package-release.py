#!/usr/bin/env python3
"""Create deterministic tar.gz or zip archives for an ncgo release tree."""

from __future__ import annotations

import argparse
import gzip
import os
from pathlib import Path
import tarfile
import time
import zipfile


def paths(root: Path) -> list[Path]:
    return sorted(root.rglob("*"), key=lambda path: path.relative_to(root.parent).as_posix())


def tar_gz(root: Path, output: Path, epoch: int) -> None:
    with output.open("wb") as raw:
        with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=epoch) as compressed:
            with tarfile.open(fileobj=compressed, mode="w", format=tarfile.PAX_FORMAT) as archive:
                for path in [root, *paths(root)]:
                    arcname = path.relative_to(root.parent).as_posix()
                    info = archive.gettarinfo(str(path), arcname)
                    info.uid = 0
                    info.gid = 0
                    info.uname = ""
                    info.gname = ""
                    info.mtime = epoch
                    if path.is_file():
                        with path.open("rb") as body:
                            archive.addfile(info, body)
                    else:
                        archive.addfile(info)


def zip_archive(root: Path, output: Path, epoch: int) -> None:
    timestamp = time.gmtime(max(epoch, 315532800))[:6]  # ZIP starts at 1980-01-01.
    with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for path in [root, *paths(root)]:
            name = path.relative_to(root.parent).as_posix()
            if path.is_dir():
                name += "/"
            info = zipfile.ZipInfo(name, timestamp)
            info.create_system = 3
            mode = path.stat().st_mode & 0o777
            info.external_attr = (mode | (0o040000 if path.is_dir() else 0o100000)) << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(info, b"" if path.is_dir() else path.read_bytes())


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--format", required=True, choices=("tar.gz", "zip"))
    parser.add_argument("--epoch", required=True, type=int)
    args = parser.parse_args()

    args.output.parent.mkdir(parents=True, exist_ok=True)
    if args.format == "tar.gz":
        tar_gz(args.root, args.output, args.epoch)
    else:
        zip_archive(args.root, args.output, args.epoch)


if __name__ == "__main__":
    main()

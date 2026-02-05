import argparse
import sys
from pathlib import Path

import piexif


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("image_file")
    args = parser.parse_args()

    image_file = Path(args.image_file)
    try:
        exif = piexif.load(str(image_file))
    except FileNotFoundError as exc:
        print(f"Image file not found: {image_file}", file=sys.stderr)
        raise SystemExit(1) from exc
    except piexif.InvalidImageDataError as exc:
        print(
            f"Invalid image data in {image_file} ({type(exc).__name__}): {exc}",
            file=sys.stderr,
        )
        raise SystemExit(1) from exc
    except Exception as exc:
        print(
            f"Failed to read EXIF data from {image_file} ({type(exc).__name__}): {exc}",
            file=sys.stderr,
        )
        raise SystemExit(1) from exc

    try:
        description = exif["0th"][piexif.ImageIFD.ImageDescription]
    except KeyError as exc:
        print(
            f"Missing ImageDescription EXIF field in {image_file}",
            file=sys.stderr,
        )
        raise SystemExit(1) from exc
    if isinstance(description, bytes):
        print(description.decode())
    else:
        print(str(description))


if __name__ == "__main__":
    main()

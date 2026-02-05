import argparse
from pathlib import Path

import piexif


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("image_file")
    args = parser.parse_args()

    image_file = Path(args.image_file)
    exif = piexif.load(str(image_file))
    description = exif["0th"][piexif.ImageIFD.ImageDescription]
    if isinstance(description, bytes):
        print(description.decode())
    else:
        print(str(description))


if __name__ == "__main__":
    main()

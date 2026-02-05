import argparse
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--template", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--album-title", required=True)
    parser.add_argument("--image-url", required=True)
    args = parser.parse_args()

    template = Path(args.template).read_text(encoding="utf-8")
    rendered = (
        template.replace("{{ALBUM_TITLE}}", args.album_title).replace(
            "{{IMAGE_URL}}",
            args.image_url,
        )
    )
    Path(args.output).write_text(rendered, encoding="utf-8")


if __name__ == "__main__":
    main()

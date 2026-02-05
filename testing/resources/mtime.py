import argparse
import os


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("file_path")
    args = parser.parse_args()
    print(int(os.path.getmtime(args.file_path)))


if __name__ == "__main__":
    main()

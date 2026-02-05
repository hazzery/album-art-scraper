import argparse
import functools
import http.server
import socketserver
import sys
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--directory", required=True)
    parser.add_argument("--port", type=int, default=0)
    parser.add_argument("--port-file", required=True)
    args = parser.parse_args()

    handler = functools.partial(
        http.server.SimpleHTTPRequestHandler,
        directory=args.directory,
    )

    try:
        with socketserver.TCPServer(("127.0.0.1", args.port), handler) as server:
            Path(args.port_file).write_text(
                str(server.server_address[1]),
                encoding="utf-8",
            )
            server.serve_forever()
    except OSError as exc:
        print(
            "Failed to start test server (possible port conflict or permissions): "
            f"{exc}",
            file=sys.stderr,
        )
        raise SystemExit(1) from exc


if __name__ == "__main__":
    main()

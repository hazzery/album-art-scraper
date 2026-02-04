#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
temp_dir="$(mktemp -d)"
album_code_length=11
album_page="$temp_dir/album.html"
image_path="$temp_dir/image.jpg"
links_file="$temp_dir/links.txt"
server_pid=""

cleanup() {
  [[ -n "$server_pid" ]] && kill "$server_pid"
  rm -rf "$temp_dir"
}
trap cleanup EXIT

printf '%s' '/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAARCAABAAEDASIAAhEBAxEB/8QAHwAAAQUBAQEBAQEAAAAAAAAAAAECAwQFBgcICQoL/8QAtRAAAgEDAwIEAwUFBAQAAAF9AQIDAAQRBRIhMUEGE1FhByJxFDKBkaEII0KxwRVS0fAkM2JyggkKFhcYGRolJicoKSo0NTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqDhIWGh4iJipKTlJWWl5iZmqKjpKWmp6ipqrKztLW2t7i5usLDxMXGx8jJytLT1NXW19jZ2uHi4+Tl5ufo6erx8vP09fb3+Pn6/8QAHwEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoL/8QAtREAAgECBAQDBAcFBAQAAQJ3AAECAxEEBSExBhJBUQdhcRMiMoEIFEKRobHBCSMzUvAVYnLRChYkNOEl8RcYGRomJygpKjU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6goOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4uPk5ebn6Onq8vP09fb3+Pn6/9oADAMBAAIRAxEAPwDi6KKK+ZP3E//Z' | base64 -d > "$image_path"

port="$(python - <<'PY'
import socket
with socket.socket() as s:
    s.bind(("", 0))
    print(s.getsockname()[1])
PY
)"
python -m http.server "$port" --bind 127.0.0.1 --directory "$temp_dir" >/dev/null 2>&1 &
server_pid=$!

python - <<PY
from pathlib import Path

image_path = Path(r"$image_path")
album_page = Path(r"$album_page")
port = "$port"
html = f"""
<html>
  <head>
    <meta name="title" content="Test Album" />
    <meta property="og:title" content="Test Album" />
    <meta property="og:image" content="http://127.0.0.1:{port}/{image_path.name}" />
  </head>
  <body></body>
</html>
"""
album_page.write_text(html, encoding="utf-8")
PY

for _ in {1..20}; do
  if curl -s "http://127.0.0.1:$port/album.html" >/dev/null; then
    break
  fi
  sleep 0.1
done

album_link="http://127.0.0.1:$port/album.html?code=AAAAAAAAAAA"
printf '%s' "$album_link" > "$links_file"

if [[ ${#album_link} -lt $album_code_length ]]; then
  echo "Album link too short for album code extraction." >&2
  exit 1
fi
album_code="${album_link: -$album_code_length}"

function assert_equals() {
  local expected="$1"
  local actual="$2"
  local message="${3:-}"
  if [[ "$expected" != "$actual" ]]; then
    echo "Expected '$expected' but got '$actual'. $message" >&2
    exit 1
  fi
}

function assert_nonzero_exit() {
  local cmd="$1"
  if eval "$cmd" >/dev/null 2>&1; then
    echo "Expected non-zero exit: $cmd" >&2
    exit 1
  fi
}

function ensure_exif_description() {
  local image_file="$1"
  local expected="$2"
  local description
  description="$(python - <<PY
import piexif
from pathlib import Path

image_file = Path(r"$image_file")
exif = piexif.load(str(image_file))
description = exif["0th"][piexif.ImageIFD.ImageDescription]
if isinstance(description, bytes):
    print(description.decode())
else:
    print(str(description))
PY
)"
  if [[ "${description:0:1}" == "=" ]]; then
    # little_exif prefixes "=" to ASCII ImageDescription values.
    description="${description:1}"
  fi
  assert_equals "$expected" "$description" "EXIF ImageDescription mismatch for $image_file"
}

function verify_skip_download() {
  local image_file="$1"
  local cmd="$2"
  local expected_code="$3"
  local before_time
  local after_time
  before_time="$(python - <<PY
import os
print(int(os.path.getmtime(r"$image_file")))
PY
)"
  eval "$cmd"
  after_time="$(python - <<PY
import os
print(int(os.path.getmtime(r"$image_file")))
PY
)"
  assert_equals "$before_time" "$after_time" "Image was re-downloaded unexpectedly."
  if [[ -n "$expected_code" ]]; then
    ensure_exif_description "$image_file" "$expected_code"
  fi
}

function test_python() {
  local image_dir="$temp_dir/python_album_arts"
  local override_dir="$temp_dir/python_override"
  local override_links="$temp_dir/python_links.txt"
  local defaults_dir="$temp_dir/python_defaults"

  python -m pip install --quiet --disable-pip-version-check aiohttp beautifulsoup4 piexif

  assert_nonzero_exit "python $root_dir/python/youtube-music-album-art-downloader.py --links-file $temp_dir/missing.txt"

  rm -rf "$image_dir"
  python "$root_dir/python/youtube-music-album-art-downloader.py" --links-file "$links_file" --image-directory "$image_dir"
  [[ -d "$image_dir" ]] || { echo "Python did not create image directory" >&2; exit 1; }
  [[ -f "$image_dir/Test Album.jpg" ]] || { echo "Python did not download image" >&2; exit 1; }
  ensure_exif_description "$image_dir/Test Album.jpg" "$album_code"
  verify_skip_download "$image_dir/Test Album.jpg" "python $root_dir/python/youtube-music-album-art-downloader.py --links-file $links_file --image-directory $image_dir" "$album_code"

  rm -rf "$override_dir"
  printf '%s' "$album_link" > "$override_links"
  python "$root_dir/python/youtube-music-album-art-downloader.py" -l "$override_links" -d "$override_dir"
  [[ -d "$override_dir" ]] || { echo "Python override did not create directory" >&2; exit 1; }
  [[ -f "$override_dir/Test Album.jpg" ]] || { echo "Python override did not download image" >&2; exit 1; }
  ensure_exif_description "$override_dir/Test Album.jpg" "$album_code"

  rm -rf "$defaults_dir"
  mkdir -p "$defaults_dir"
  printf '%s' "$album_link" > "$defaults_dir/links.txt"
  (cd "$defaults_dir" && python "$root_dir/python/youtube-music-album-art-downloader.py")
  [[ -d "$defaults_dir/album_arts" ]] || { echo "Python defaults did not create directory" >&2; exit 1; }
  [[ -f "$defaults_dir/album_arts/Test Album.jpg" ]] || { echo "Python defaults did not download image" >&2; exit 1; }
  ensure_exif_description "$defaults_dir/album_arts/Test Album.jpg" "$album_code"
}

function test_go() {
  local image_dir="$temp_dir/go_album_arts"
  local override_dir="$temp_dir/go_override"
  local override_links="$temp_dir/go_links.txt"
  local defaults_dir="$temp_dir/go_defaults"

  (cd "$root_dir/go" && go build -o "$temp_dir/go_downloader")

  assert_nonzero_exit "$temp_dir/go_downloader --links-file $temp_dir/missing.txt"

  rm -rf "$image_dir"
  "$temp_dir/go_downloader" --links-file "$links_file" --image-directory "$image_dir"
  [[ -d "$image_dir" ]] || { echo "Go did not create image directory" >&2; exit 1; }
  [[ -f "$image_dir/Test Album.jpg" ]] || { echo "Go did not download image" >&2; exit 1; }
  ensure_exif_description "$image_dir/Test Album.jpg" "$album_code"
  verify_skip_download "$image_dir/Test Album.jpg" "$temp_dir/go_downloader --links-file $links_file --image-directory $image_dir" "$album_code"

  rm -rf "$override_dir"
  printf '%s' "$album_link" > "$override_links"
  "$temp_dir/go_downloader" -l "$override_links" -d "$override_dir"
  [[ -d "$override_dir" ]] || { echo "Go override did not create directory" >&2; exit 1; }
  [[ -f "$override_dir/Test Album.jpg" ]] || { echo "Go override did not download image" >&2; exit 1; }
  ensure_exif_description "$override_dir/Test Album.jpg" "$album_code"

  rm -rf "$defaults_dir"
  mkdir -p "$defaults_dir"
  printf '%s' "$album_link" > "$defaults_dir/links.txt"
  (cd "$defaults_dir" && "$temp_dir/go_downloader")
  [[ -d "$defaults_dir/album_arts" ]] || { echo "Go defaults did not create directory" >&2; exit 1; }
  [[ -f "$defaults_dir/album_arts/Test Album.jpg" ]] || { echo "Go defaults did not download image" >&2; exit 1; }
  ensure_exif_description "$defaults_dir/album_arts/Test Album.jpg" "$album_code"
}

function test_rust() {
  local image_dir="$temp_dir/rust_album_arts"
  local override_dir="$temp_dir/rust_override"
  local override_links="$temp_dir/rust_links.txt"
  local defaults_dir="$temp_dir/rust_defaults"

  (cd "$root_dir/rust" && cargo build --quiet)

  assert_nonzero_exit "$root_dir/rust/target/debug/ytm-album-art-downloader --links-file $temp_dir/missing.txt"

  rm -rf "$image_dir"
  "$root_dir/rust/target/debug/ytm-album-art-downloader" --links-file "$links_file" --image-directory "$image_dir"
  [[ -d "$image_dir" ]] || { echo "Rust did not create image directory" >&2; exit 1; }
  [[ -f "$image_dir/Test Album.jpg" ]] || { echo "Rust did not download image" >&2; exit 1; }
  ensure_exif_description "$image_dir/Test Album.jpg" "$album_code"
  verify_skip_download "$image_dir/Test Album.jpg" "$root_dir/rust/target/debug/ytm-album-art-downloader --links-file $links_file --image-directory $image_dir" "$album_code"

  rm -rf "$override_dir"
  printf '%s' "$album_link" > "$override_links"
  "$root_dir/rust/target/debug/ytm-album-art-downloader" -l "$override_links" -d "$override_dir"
  [[ -d "$override_dir" ]] || { echo "Rust override did not create directory" >&2; exit 1; }
  [[ -f "$override_dir/Test Album.jpg" ]] || { echo "Rust override did not download image" >&2; exit 1; }
  ensure_exif_description "$override_dir/Test Album.jpg" "$album_code"

  rm -rf "$defaults_dir"
  mkdir -p "$defaults_dir"
  printf '%s' "$album_link" > "$defaults_dir/links.txt"
  (cd "$defaults_dir" && "$root_dir/rust/target/debug/ytm-album-art-downloader")
  [[ -d "$defaults_dir/album_arts" ]] || { echo "Rust defaults did not create directory" >&2; exit 1; }
  [[ -f "$defaults_dir/album_arts/Test Album.jpg" ]] || { echo "Rust defaults did not download image" >&2; exit 1; }
  ensure_exif_description "$defaults_dir/album_arts/Test Album.jpg" "$album_code"
}

test_python
test_go
test_rust
echo "Integration tests passed."

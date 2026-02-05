#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
resources_dir="$root_dir/testing/resources"
temp_dir="$(mktemp -d)"
album_code_length=11
album_page="$temp_dir/album.html"
image_path="$temp_dir/album.jpg"
links_file="$temp_dir/links.txt"
port_file="$temp_dir/port.txt"
server_pid=""
bin_dir="$root_dir/testing/bin"
python_exec="$root_dir/python/.venv/bin/python"

cleanup() {
  [[ -n "$server_pid" ]] && kill "$server_pid"
  rm -rf "$temp_dir"
}
trap cleanup EXIT

cp "$resources_dir/album.jpg" "$image_path"

if [[ ! -x "$python_exec" ]]; then
  echo "Missing Python virtualenv or dependencies. Run: make -C \"$root_dir/python\" export" >&2
  exit 1
fi

"$python_exec" "$resources_dir/serve_assets.py" \
  --directory "$temp_dir" \
  --port 0 \
  --port-file "$port_file" \
  >/dev/null 2>&1 &
server_pid=$!

for _ in {1..20}; do
  if [[ -f "$port_file" ]]; then
    port="$(cat "$port_file")"
    break
  fi
  sleep 0.1
done

if [[ -z "${port:-}" ]]; then
  echo "Failed to determine server port. Check '$port_file' or verify the test server started." >&2
  exit 1
fi

"$python_exec" "$resources_dir/render_album_html.py" \
  --template "$resources_dir/album_page.html" \
  --output "$album_page" \
  --album-title "Test Album" \
  --image-url "http://127.0.0.1:$port/$(basename "$image_path")"

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
  description="$("$python_exec" "$resources_dir/exif_description.py" "$image_file")"
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
  before_time="$("$python_exec" "$resources_dir/mtime.py" "$image_file")"
  eval "$cmd"
  after_time="$("$python_exec" "$resources_dir/mtime.py" "$image_file")"
  assert_equals "$before_time" "$after_time" "Image was re-downloaded unexpectedly."
  if [[ -n "$expected_code" ]]; then
    ensure_exif_description "$image_file" "$expected_code"
  fi
}

function run_tests() {
  local name="$1"
  local app="$2"
  local image_dir="$temp_dir/${name}_album_arts"
  local override_dir="$temp_dir/${name}_override"
  local override_links="$temp_dir/${name}_links.txt"
  local defaults_dir="$temp_dir/${name}_defaults"

  if [[ ! -x "$app" ]]; then
    echo "Expected executable at $app. Run: make -C $name export from the repository root." >&2
    exit 1
  fi

  assert_nonzero_exit "$app --links-file $temp_dir/missing.txt"

  rm -rf "$image_dir"
  "$app" --links-file "$links_file" --image-directory "$image_dir"
  [[ -d "$image_dir" ]] || { echo "$name did not create image directory" >&2; exit 1; }
  [[ -f "$image_dir/Test Album.jpg" ]] || { echo "$name did not download image" >&2; exit 1; }
  ensure_exif_description "$image_dir/Test Album.jpg" "$album_code"
  verify_skip_download "$image_dir/Test Album.jpg" "$app --links-file $links_file --image-directory $image_dir" "$album_code"

  rm -rf "$override_dir"
  printf '%s' "$album_link" > "$override_links"
  "$app" -l "$override_links" -d "$override_dir"
  [[ -d "$override_dir" ]] || { echo "$name override did not create directory" >&2; exit 1; }
  [[ -f "$override_dir/Test Album.jpg" ]] || { echo "$name override did not download image" >&2; exit 1; }
  ensure_exif_description "$override_dir/Test Album.jpg" "$album_code"

  rm -rf "$defaults_dir"
  mkdir -p "$defaults_dir"
  printf '%s' "$album_link" > "$defaults_dir/links.txt"
  (cd "$defaults_dir" && "$app")
  [[ -d "$defaults_dir/album_arts" ]] || { echo "$name defaults did not create directory" >&2; exit 1; }
  [[ -f "$defaults_dir/album_arts/Test Album.jpg" ]] || { echo "$name defaults did not download image" >&2; exit 1; }
  ensure_exif_description "$defaults_dir/album_arts/Test Album.jpg" "$album_code"
}

for implementation in python go rust; do
  run_tests "$implementation" "$bin_dir/${implementation}-downloader"
done
echo "Integration tests passed."

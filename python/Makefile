ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/..)
BIN_DIR := $(ROOT_DIR)/testing/bin
VENV := $(ROOT_DIR)/python/.venv
SCRIPT := $(ROOT_DIR)/python/youtube-music-album-art-downloader.py
OUTPUT := $(BIN_DIR)/python-downloader

.PHONY: export

export:
	python -m venv $(VENV)
	$(VENV)/bin/pip install --quiet --disable-pip-version-check aiohttp beautifulsoup4 piexif
	@mkdir -p $(BIN_DIR)
	printf '%s\n' '#!/usr/bin/env bash' '"$(VENV)/bin/python" "$(SCRIPT)" "$$@"' > $(OUTPUT)
	chmod +x $(OUTPUT)

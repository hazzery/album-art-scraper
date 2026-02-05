ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/..)
BIN_DIR := $(ROOT_DIR)/testing/bin
OUTPUT := $(BIN_DIR)/rust-downloader
SOURCE := $(ROOT_DIR)/rust/target/debug/ytm-album-art-downloader

.PHONY: export

export:
	cargo build --quiet
	@mkdir -p $(BIN_DIR)
	cp $(SOURCE) $(OUTPUT)

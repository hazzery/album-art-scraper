ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/..)
BIN_DIR := $(ROOT_DIR)/testing/bin
OUTPUT := $(BIN_DIR)/go-downloader

.PHONY: export

export:
	@mkdir -p $(BIN_DIR)
	go build -o $(OUTPUT)

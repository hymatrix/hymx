.PHONY: build build-linux

build:
	go build -o ./build/hymx ./cmd

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./build/hymx-linux ./cmd
# SPDX-License-Identifier: EUPL-1.2

# Build a static binary into bin/myul with version metadata.
build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev) -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/myul ./cmd/myul

# Run the full test suite (offline, no network).
test:
	go test ./...

# Run golangci-lint over the whole module.
lint:
	golangci-lint run ./...

# Build and run myul.
run: build
	./bin/myul

# Regenerate the README promotional GIF (needs deno, rsvg-convert, ffmpeg).
demo:
	deno run -A docs/assets/demo/demo.ts

# Remove build artifacts.
clean:
	rm -rf bin

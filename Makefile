VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

.PHONY: all build build-arm64 build-web dev dev-web test clean

all: build

build:
	go build $(LDFLAGS) -o teslausb ./cmd/teslausb

build-arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o teslausb-arm64 ./cmd/teslausb

test:
	go test ./...

dev: build
	./teslausb -config config.yaml.example

build-web:
	cd web && npm run build
	mkdir -p internal/web/static
	cp -r web/dist/* internal/web/static/

dev-web:
	cd web && npm run dev

clean:
	rm -f teslausb teslausb-arm64

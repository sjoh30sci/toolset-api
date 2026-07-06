.PHONY: build build-cli test test-search test-files test-e2e test-all run clean docker-build docker-up docker-down compose-config help release package sdk-generate openapi docs install-dev

help:
	@echo "Toolset API - Available targets:"
	@echo "  make build         - Build Go gateway binary into bin/"
	@echo "  make build-cli     - Build the toolset CLI into bin/"
	@echo "  make test          - Run unit tests"
	@echo "  make test-search   - Run search handler tests"
	@echo "  make test-files    - Run files handler tests"
	@echo "  make test-e2e      - Run docker-compose end-to-end smoke test"
	@echo "  make test-all      - Run unit + search + files tests"
	@echo "  make run           - Run gateway locally (requires services running)"
	@echo "  make docker-build  - Build all Docker images"
	@echo "  make docker-up     - docker-compose up -d"
	@echo "  make docker-down   - docker-compose down"
	@echo "  make compose-config- Validate docker-compose syntax"
	@echo "  make openapi       - Generate gateway/openapi.json from the gateway"
	@echo "  make sdk-generate  - Generate TypeScript + Python SDKs from OpenAPI"
	@echo "  make package       - Build a portable distribution tarball"
	@echo "  make release       - Run GoReleaser (requires a git tag)"
	@echo "  make docs          - List documentation files"
	@echo "  make install-dev   - Install release/codegen tooling"
	@echo "  make clean         - Clean binaries and build artifacts"

build:
	cd gateway && go build -o ../bin/gateway -ldflags "-X main.Version=$(shell git describe --tags --always 2>/dev/null || echo dev)" .

build-cli:
	cd cli && go build -o ../bin/toolset -ldflags "-X main.Version=$(shell git describe --tags --always 2>/dev/null || echo dev)" .

test:
	cd gateway && go test ./...

test-search:
	cd gateway && go test ./internal/handlers -run TestSearch -v

test-files:
	cd gateway && go test ./internal/handlers -run TestFiles -v

test-e2e:
	bash scripts/test-e2e.sh

test-all: test test-search test-files

run: build
	./bin/gateway

docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

compose-config:
	docker-compose config

openapi:
	cd gateway && go run . --openapi-spec > openapi.json
	@echo "wrote gateway/openapi.json"

sdk-generate: openapi
	docker run --rm -v "$(PWD)":/local openapitools/openapi-generator-cli:latest generate \
		-i /local/gateway/openapi.json \
		-g typescript-axios \
		-o /local/sdk/ts
	docker run --rm -v "$(PWD)":/local openapitools/openapi-generator-cli:latest generate \
		-i /local/gateway/openapi.json \
		-g python \
		-o /local/sdk/py

package: build-cli
	./bin/toolset package toolset-latest.tar.gz

release:
	goreleaser release --clean

docs:
	@echo "Docs are in docs/ directory"
	@ls -la docs/

install-dev:
	@echo "Install GoReleaser: https://goreleaser.com/install/"
	@echo "  macOS:  brew install goreleaser"
	@echo "  Linux:  see https://goreleaser.com/install/#apt"
	go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest

clean:
	rm -f bin/gateway bin/gateway.exe bin/toolset bin/toolset.exe
	rm -rf bin/

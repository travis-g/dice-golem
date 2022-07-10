GOFMT_FILES?=$$(find . -name '*.go' | grep -v pb.go | grep -vE "vendor/")

.PHONY: default
default: fmt test build

.PHONY: fmt
fmt:
	@echo "--> Formatting source files..."
	gofmt -w $(GOFMT_FILES)

.PHONY: test
test: fmt
	@echo "--> Testing..."
	go test ./...

.PHONY: lint
lint:
	@echo "--> Running linter..."
	@golangci-lint -v run

.PHONY: build
build: fmt test
	@echo "--> Building!"
	go build -ldflags="-s -w" -o dice-golem

.PHONY: prod
prod:
	@echo "--> Building Production bot..."
	mkdir -p dist/
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/dice-golem

.PHONY: debug
debug: build
	@echo "--> Running in debug mode..."
	GOLEM_DEBUG=true GOLEM_RECENT=4h ./dice-golem

.PHONY: clean
clean:
	@echo "--> Cleaning..."
	rm -rf dice-golem dice-golem.exe dist

.PHONY: redis
redis:
	@echo "--> Starting Redis cache..."
	docker run -p 6379:6379 -d redis

.PHONY: docs
docs:
	dot -Tsvg -Gfontname="sans-serif" -Nfontname="sans-serif" -Efontname="sans-serif" docs/architecture.dot >docs/architecture.svg

.PHONY: docker-build
docker-build:
	@echo "--> Building container..."
	docker build -t dice-golem:$$(git rev-parse --abbrev-ref HEAD` . -f Dockerfile)

.PHONY: docker-run
docker-run: docker-build
	@echo "--> Running bot (Docker)..."
	docker run --rm --env-file .env \
		-t dice-golem:$$(git rev-parse --abbrev-ref HEAD)

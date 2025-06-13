GOFMT_FILES?=$$(find . -name '*.go' | grep -v pb.go | grep -vE "vendor/")

.PHONY: default
default: fmt test build

.PHONY: fmt
fmt:
	@echo "--> Formatting source files..."
	go1.24.4 fmt -s -w $(GOFMT_FILES)

.PHONY: test
test:
	@echo "--> Testing..."
	go1.24.4 test -cover ./...

.PHONY: lint
lint:
	@echo "--> Running linter..."
	@golangci-lint -v run

.PHONY: build
build: fmt test
	@echo "--> Building!"
	go1.24.4 build -ldflags="-s -w" -o dice-golem

.PHONY: dist
dist:
	@echo "--> Building Production binary..."
	mkdir -p dist/
	GOOS=linux GOARCH=amd64 go1.24.4 build -ldflags="-s -w" -o dist/dice-golem

.PHONY: debug
debug: dev
.PHONY: dev
dev: test
	@echo "--> Running in dev/debug mode..."
	go1.24.4 build -ldflags="-s -w" -o dice-golem
	GOLEM_DEBUG=true GOLEM_RECENT=4h ./dice-golem

.PHONY: clean
clean:
	@echo "--> Cleaning..."
	rm -rf dice-golem dice-golem.exe dist/dice-golem

.PHONY: redis
redis:
	@echo "--> Starting Redis container..."
	docker run -p 6379:6379 -d redis

.PHONY: docs
docs:
	dot -Tsvg -Gfontname="sans-serif" -Nfontname="sans-serif" -Efontname="sans-serif" docs/architecture.dot >docs/architecture.svg

test:
	go test ./...
.PHONY: test

coverage:
	go test -coverprofile=cover.out ./...
.PHONY: coverage

generate:
	go generate ./...
.PHONY: generate

examples:
	cd examples && go generate && go mod tidy
.PHONY: examples

test_examples:
	cd examples && go test ./...
.PHONY: test_examples

tidy:
	go mod tidy
.PHONY: tidy

tidy_examples:
	cd examples && go mod tidy
.PHONY: tidy_examples

tidy_all: tidy tidy_examples
.PHONY: tidy_all

lint:
	golangci-lint run ./...
.PHONY: lint

commit_gen:
	git add ./examples ./internal/integration/*/*_gen*.go
	git commit -m "chore: commit generated files"
.PHONY: commit_gen

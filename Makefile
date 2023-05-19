.DEFAULT_GOAL := lint

format:
	gofmt -w .

gofumpt:
	gofumpt -w .

lint: format gofumpt
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin
	golangci-lint run ./...

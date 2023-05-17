.DEFAULT_GOAL := lint

format:
	gofmt -s -w .

lint: format
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin
	golangci-lint run .
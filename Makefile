.PHONY: build test fmt vet tidy

build:
	go build ./cmd/server

test:
	go test ./...

fmt:
	gofmt -w cmd pkg

vet:
	go vet ./...

tidy:
	go mod tidy

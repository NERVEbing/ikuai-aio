.PHONY: build test vet fmt check-fmt

build:
	CGO_ENABLED=0 go build -trimpath -o bin/ikuai-aio ./cmd

test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w api config cmd exporter job internal

check-fmt:
	@test -z "$$(gofmt -l api config cmd exporter job internal)"

.PHONY: generate run build test

generate:
	go tool templ generate

run: generate
	go run ./...

build: generate
	go build

test: generate
	go test ./...

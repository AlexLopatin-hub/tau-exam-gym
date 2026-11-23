.PHONY: generate run build test

generate:
	go tool templ generate
	go generate ./internal/exam

run: generate
	go run .

build: generate
	go build

test: generate
	go test ./...

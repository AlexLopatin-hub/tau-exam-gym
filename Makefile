run:
	go mod tidy
	go tool templ generate
	go run ./...

build:
	go mod tidy
	go tool templ generate
	go build

tools:
	go install github.com/maxbrunsfeld/counterfeiter/v6
	go install github.com/cespare/reflex@latest
.PHONY: tools

generate:
	go generate ./...
.PHONY: generate

run-chat:
	go run main.go
.PHONY: run-chat
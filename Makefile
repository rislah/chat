.PHONY: tools
tools:
	go install github.com/maxbrunsfeld/counterfeiter/v6

.PHONY: generate
generate:
	go generate ./...
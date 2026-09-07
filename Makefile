.PHONY: run test vet fmt tidy

run:
	go run ./cmd/bot

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

tidy:
	go mod tidy

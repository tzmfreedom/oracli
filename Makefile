.PHONY: run
run: format
	go run .

.PHONY: format
format:
	gofmt -w .

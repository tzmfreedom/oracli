.PHONY: run
run: format
	go run .

.PHONY: format
format:
	gofmt -w .

.PHONY: docker/up
docker/up:
	docker-compose up

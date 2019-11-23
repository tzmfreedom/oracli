.PHONY: run
run: format
	go run . -u system -H localhost -p 1521 -s ORCLCDB

.PHONY: format
format:
	gofmt -w .

.PHONY: docker/up
docker/up:
	docker-compose up

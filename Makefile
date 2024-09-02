.PHONY: build

build:
	go build -o aliassrv -v ./cmd/main.go

.PHONY: test
test:
	go clean -testcache
	go test -v -race -cover -timeout 30s ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: mocks
mocks:
	 go generate ./... -v

.PHONY: docker
docker:
	docker build . -t alias:v1.0.0alpha

.PHONY: migrate_up
migrate_up:
	migrate -source file://migrations/mongodb -database mongodb://root:root@localhost:27017/admin up

.PHONY: migrate_down
migrate_down:
	migrate -source file://migrations/mongodb -database mongodb://root:root@localhost:27017/admin down

.DEFAULT_GOAL := build
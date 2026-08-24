.PHONY: build test vet fmt up down logs

build:
	cd backend && go build ./...

test:
	cd backend && go test ./...

vet:
	cd backend && go vet ./...

fmt:
	cd backend && gofmt -l .

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

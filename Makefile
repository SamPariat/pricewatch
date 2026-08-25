.PHONY: build test vet fmt swagger up down logs

build:
	cd backend && go build ./...

# Regenerate the OpenAPI/Swagger spec from handler doc comments after
# changing any route — see internal/httpapi/doc.go for the annotation
# format. Served at /api/docs (UI) and /api/docs/openapi.json (raw spec).
swagger:
	cd backend && go run github.com/swaggo/swag/cmd/swag@v1.16.6 init \
		-g internal/httpapi/doc.go -o internal/httpapi/docs \
		--outputTypes json,yaml --parseInternal --parseDependency

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

# Build context is the monorepo root (not backend/) — see docker-compose.yml
# — so this image can also COPY locales/, the shared translation source
# also read directly by web/Dockerfile's Next.js build. See
# backend/internal/i18n's own doc comment for why it isn't go:embed'd
# instead: `//go:embed` cannot reach outside this module's own directory
# tree, and locales/ deliberately lives above it, shared with web/.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
COPY locales/ /locales/
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]

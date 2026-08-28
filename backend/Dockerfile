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

# Not distroless anymore — the Airbnb scraper (internal/providers/airbnb)
# needs a real Chromium binary, which means a package manager to install
# it. This is a genuine regression in image size (~300-400MB vs
# distroless's <30MB) and attack surface; accepted as the cost of
# scraping a site with no API. CHROME_PATH tells chromedp exactly where
# to find it rather than relying on auto-discovery.
FROM alpine:3.20
RUN apk add --no-cache chromium ca-certificates tzdata
ENV CHROME_PATH=/usr/bin/chromium-browser
COPY --from=build /out/server /server
COPY locales/ /locales/
RUN adduser -D -u 65532 nonroot
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]

FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go test ./...

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /out/crawl-proxy .

FROM alpine:3.22

LABEL org.opencontainers.image.source="https://github.com/byfebian/crawl4ai-proxy"
LABEL org.opencontainers.image.description="A proxy that enables OpenWebUI to talk to Crawl4AI with advanced features"

RUN addgroup -S app && adduser -S -G app app

COPY --from=builder /out/crawl-proxy /usr/local/bin/crawl-proxy

EXPOSE 8000
USER app
ENTRYPOINT ["/usr/local/bin/crawl-proxy"]
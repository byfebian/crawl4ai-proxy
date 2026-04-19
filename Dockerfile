# v0.0.3 change:
# Use an explicit Go toolchain image and run tests in the builder stage so
# broken releases fail during image build instead of at runtime.
FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY . .

# v0.0.3 change:
# Run unit tests inside the container build to keep CI and image behavior aligned.
RUN go test ./...

# v0.0.3 change:
# Build a small static binary and respect buildx target platform variables for
# multi-arch publishing (linux/amd64, linux/arm64).
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /out/crawl-proxy .

FROM alpine:3.22

# v0.0.3 change:
# Update OCI source label to this fork repository.
LABEL org.opencontainers.image.source="https://github.com/byfebian/crawl4ai-proxy"
LABEL org.opencontainers.image.description="A simple proxy that enables OpenWebUI to talk to Crawl4AI"

# v0.0.3 change:
# Run as non-root for safer default container posture.
RUN addgroup -S app && adduser -S -G app app

COPY --from=builder /out/crawl-proxy /usr/local/bin/crawl-proxy

EXPOSE 8000
USER app
ENTRYPOINT ["/usr/local/bin/crawl-proxy"]

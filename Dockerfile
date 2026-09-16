# syntax=docker/dockerfile:1.7
# ─────────────────────────────────────────────────────────────────────────────
# Kampong — multi-stage build producing a static, distroless container.
# Build:   docker build -t kampong:latest --build-arg VERSION=$(git rev-parse --short HEAD 2>/dev/null || echo dev) .
# Run:     docker run --rm -p 9090:9090 -v $PWD/data:/app/data -v $PWD/config.yaml:/app/config.yaml:ro kampong:latest
# ─────────────────────────────────────────────────────────────────────────────

ARG GO_VERSION=1.25

# ── Stage 1: build ──────────────────────────────────────────────────────────
FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /src

# Cache modules first
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build a fully static binary so distroless/static can run it.
ARG VERSION=dev
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

RUN go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/kampong \
    ./cmd/server

# ── Stage 2: runtime ────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/kampong /app/kampong
COPY web /app/web
COPY config.example.yaml /app/config.example.yaml

# Distroless already ships with a non-root user (uid 65532)
USER nonroot:nonroot

EXPOSE 9090

# data/ is expected to be mounted at runtime; SQLite file lives there.
VOLUME ["/app/data"]

ENTRYPOINT ["/app/kampong"]
CMD ["-config", "/app/config.yaml"]

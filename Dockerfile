# syntax=docker/dockerfile:1.7
FROM golang:1.23 AS builder-base

ARG TARGETOS=linux
ARG TARGETARCH
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

FROM builder-base AS server-builder
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM builder-base AS agent-builder
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/agent ./cmd/agent

FROM gcr.io/distroless/static-debian12:nonroot AS server
COPY --from=server-builder /out/server /server
ENTRYPOINT ["/server"]

FROM gcr.io/distroless/static-debian12:nonroot AS agent
COPY --from=agent-builder /out/agent /agent
ENTRYPOINT ["/agent"]

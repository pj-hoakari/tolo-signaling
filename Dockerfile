# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY gen ./gen

ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build,id=go-build-${TARGETARCH} \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/signaling ./cmd/signaling


FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/signaling /usr/local/bin/signaling

EXPOSE 8787

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/signaling"]

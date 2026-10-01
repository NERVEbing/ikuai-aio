# syntax=docker/dockerfile:1
FROM golang:1.26.8-alpine AS builder
RUN apk add --no-cache ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags='-s -w' -o /out/ikuai-aio ./cmd

FROM scratch
LABEL org.opencontainers.image.source="https://github.com/NERVEbing/ikuai-aio" \
      org.opencontainers.image.licenses="MIT"
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/ikuai-aio /ikuai-aio
USER 65532:65532
ENV TZ=Asia/Shanghai
EXPOSE 8000
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD ["/ikuai-aio", "healthcheck"]
ENTRYPOINT ["/ikuai-aio"]

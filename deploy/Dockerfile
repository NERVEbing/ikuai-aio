FROM golang:1.20-alpine AS builder

COPY . /app

WORKDIR /app

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

RUN mkdir -p bin && go build -o ./bin ./...

FROM alpine

RUN apk update && apk add tzdata

ENV TZ=Asia/Shanghai

COPY --from=builder /app/bin /app

WORKDIR /app

CMD ["./cmd"]

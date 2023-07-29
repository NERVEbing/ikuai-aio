FROM golang:1.20-alpine AS builder

COPY . /app

WORKDIR /app

ENV GO111MODULE=on \
    CGO_ENABLED=0

RUN mkdir -p bin && go build -o ./bin ./...

FROM alpine:latest

RUN apk update && apk --no-cache add tzdata

ENV TZ=Asia/Shanghai

COPY --from=builder /app/bin /app

WORKDIR /app

CMD ["./cmd"]

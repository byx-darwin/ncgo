FROM golang:1.26.5-alpine AS builder
WORKDIR /src

ENV CGO_ENABLED=0 GOOS=linux

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/app .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /out/app ./app

ENV GO_ENV=docker

EXPOSE 8080

ENTRYPOINT ["./app"]

FROM golang:1.22-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG APP_NAME=app
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

WORKDIR /app
COPY --from=builder --chown=appuser:appgroup /out/app ./app
USER appuser

ENV GO_ENV=docker

EXPOSE 8080

ENTRYPOINT ["./app"]

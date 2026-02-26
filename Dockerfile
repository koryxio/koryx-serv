# Build stage
FROM golang:1.26-alpine3.23 AS builder

RUN apk add --no-cache git make

WORKDIR /app
COPY go.mod ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=docker" -o koryx-serv

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
RUN addgroup -g 1000 koryx-serv && \
    adduser -D -u 1000 -G koryx-serv koryx-serv

WORKDIR /app
COPY --from=builder /app/koryx-serv /app/koryx-serv

RUN mkdir -p /app/public && \
    chown -R koryx-serv:koryx-serv /app

USER koryx-serv

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

ENTRYPOINT ["/app/koryx-serv"]
CMD ["-dir", "/app/public"]

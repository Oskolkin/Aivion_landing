FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 go build -o /bin/worker ./cmd/worker
RUN CGO_ENABLED=0 go build -o /bin/admin ./cmd/admin

FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget \
  && adduser -D -u 10001 appuser

WORKDIR /app
COPY --from=builder /bin/api /app/api
COPY --from=builder /bin/worker /app/worker
COPY --from=builder /bin/admin /app/admin
COPY migrations /app/migrations
# Sanitized one-off OAuth coverage evidence is intentionally retained only in
# this writable diagnostic location. It contains no Ozon payloads or tokens.
RUN mkdir -p /app/backend/debug/ozon-source-coverage \
  && chown -R appuser:appuser /app

USER appuser
EXPOSE 8081
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8081/health/live >/dev/null || exit 1
CMD ["/app/api"]

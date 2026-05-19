# syntax=docker/dockerfile:1

FROM golang:1.26.2-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /src/backend
RUN go mod download
COPY backend/ ./

RUN set -eux; \
    mkdir -p /out; \
    for bin in api worker cleanup migrate smoke admin; do \
        CGO_ENABLED=0 GOOS=linux go build \
            -ldflags="-s -w" \
            -trimpath \
            -o "/out/${bin}" "./cmd/${bin}"; \
    done


FROM alpine:3.23
WORKDIR /app/backend

RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -g 10001 -S app && \
    adduser -u 10001 -S -D -H \
        -h /app \
        -s /sbin/nologin \
        -G app app
RUN mkdir -p /data /app/backend/migrations && \
    chown -R app:app /data /app

COPY --from=builder /out/api /usr/local/bin/api
COPY --from=builder /out/worker /usr/local/bin/worker
COPY --from=builder /out/cleanup /usr/local/bin/cleanup
COPY --from=builder /out/migrate /usr/local/bin/migrate
COPY --from=builder /out/smoke /usr/local/bin/smoke
COPY --from=builder /out/admin /usr/local/bin/admin
COPY backend/migrations ./migrations

USER app:app

EXPOSE 8080
VOLUME ["/data"]

CMD ["api"]

# syntax=docker/dockerfile:1

FROM golang:1.26.2-alpine AS builder

ARG APP_VERSION=0.1.0-dev
ARG APP_COMMIT=unknown
ARG APP_BUILD_DATE=unknown

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
            -ldflags="-s -w -X 'gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/buildinfo.Version=${APP_VERSION}' -X 'gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/buildinfo.Commit=${APP_COMMIT}' -X 'gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/buildinfo.Date=${APP_BUILD_DATE}'" \
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

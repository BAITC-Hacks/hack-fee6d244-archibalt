# ---- фронт: собираем web/, если он уже есть ----
FROM node:22-alpine AS web
WORKDIR /web
COPY web/ ./
RUN if [ -f package.json ]; then \
      (if [ -f package-lock.json ]; then npm ci; else npm install; fi) && npm run build; \
    else \
      echo "web/package.json not found — frontend skipped"; mkdir -p dist; \
    fi

# ---- бэкенд ----
FROM golang:1.26-alpine AS go
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server && mkdir -p seed

# ---- финальный образ ----
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=go /out/server ./server
COPY --from=go /src/seed/ ./seed/
COPY --from=web /web/dist ./web/dist
ENV PORT=8080 SEED_DIR=/app/seed STATIC_DIR=/app/web/dist
EXPOSE 8080
CMD ["./server"]

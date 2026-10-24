# Stage 1: Build Next.js frontend export
FROM node:22-alpine AS web-builder
WORKDIR /work/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go blog-server binary
FROM golang:1.22-alpine AS go-builder
WORKDIR /work
COPY go.mod go.sum* ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/blog-server ./cmd/blog-server

# Stage 3: Minimal secure runtime image (UID 10001:10001, restricted PSS compliant)
FROM alpine:3.20 AS runtime
RUN apk add --no-cache ca-certificates && \
    addgroup -g 10001 appgroup && \
    adduser -u 10001 -G appgroup -s /sbin/nologin -D appuser && \
    mkdir -p /app/web/out /app/seed-content /var/run/blog-content /tmp && \
    chown -R 10001:10001 /app /var/run/blog-content /tmp

WORKDIR /app
COPY --from=go-builder /out/blog-server /app/blog-server
COPY --from=web-builder /work/web/out /app/web/out
COPY content/ /app/seed-content/

USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/app/blog-server"]

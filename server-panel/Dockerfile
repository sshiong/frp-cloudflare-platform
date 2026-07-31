# Build stage
FROM golang:1.21-alpine AS builder

ARG TARGET=control

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web-admin
COPY web-admin/package*.json ./
RUN npm ci
COPY web-admin/ .
RUN npm run build

# Final build stage
FROM builder AS final

# Copy frontend build output
COPY --from=frontend /app/web-admin/dist ./web-admin/dist

# Build Go binary
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /usr/local/bin/frp-panel-${TARGET} ./cmd/${TARGET}/

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 frp-panel && \
    adduser -u 1000 -G frp-panel -s /bin/sh -D frp-panel

# Create data directories
RUN mkdir -p /data /var/lib/frp-panel/router/snapshots /var/lib/frp-panel/router/certificates && \
    chown -R frp-panel:frp-panel /data /var/lib/frp-panel

COPY --from=final /usr/local/bin/frp-panel-* /usr/local/bin/

USER frp-panel

VOLUME ["/data", "/var/lib/frp-panel/router"]

EXPOSE 80 443 9000

ENTRYPOINT ["frp-panel"]
CMD ["control"]

# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Copy module files first for better layer caching
COPY go.mod ./

# Generate go.sum if missing (allows building without pre-generated go.sum)
RUN go mod download 2>/dev/null || true

# Copy source code
COPY . .

# Ensure dependencies are resolved, then build
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/server ./cmd/server

# Runtime stage
FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=UTC
WORKDIR /app
COPY --from=builder /build/server .
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["./server"]

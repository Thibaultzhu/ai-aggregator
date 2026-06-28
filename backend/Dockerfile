# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Copy module files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build from the committed module graph. Do not run go mod tidy inside Docker;
# that can pull test-only transitive modules and make local builds flaky.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/server ./cmd/server

# Runtime stage. The server is built with CGO disabled, so a scratch image is
# enough and avoids pulling an extra runtime base image on locked-down machines.
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]

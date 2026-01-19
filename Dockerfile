# Builder Stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o mock-feed ./cmd/mock-feed

# Runner Stage
FROM alpine:3.19

WORKDIR /app

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Install certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Copy binaries from builder
COPY --from=builder /app/main .
COPY --from=builder /app/mock-feed .
COPY --from=builder /app/config ./config

# Set ownership
RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

CMD ["./main"]

# ============================
# 1. Build Stage
# ============================
FROM golang:1.22-alpine AS builder

# Install git (required for go mod download)
RUN apk add --no-cache git

WORKDIR /app

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY internal/commands .

# Build a static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o bot ./cmd/bot

# ============================
# 2. Runtime Stage
# ============================
FROM alpine:3.20

WORKDIR /app

# Create non-root user
RUN adduser -D -u 10001 botuser
USER botuser

# Copy binary from builder
COPY --from=builder /app/bot /app/bot

# Environment variables (can be overridden at runtime)
ENV DISCORD_BOT_TOKEN=""
ENV DISCORD_APP_ID=""

# Run the bot
CMD ["/app/bot"]
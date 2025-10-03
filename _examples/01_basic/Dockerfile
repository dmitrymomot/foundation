FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
ARG TARGETARCH
ARG BUILDPLATFORM
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -o app .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/app .
RUN chmod +x app

# Copy migrations
# COPY --from=builder /build/internal/db/migrations ./internal/db/migrations/

EXPOSE 8080

CMD ["./app"]

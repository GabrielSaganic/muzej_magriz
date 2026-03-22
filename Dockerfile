# Stage 1: Build (Using local vendor folder)
FROM golang:1.21-alpine AS builder
WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Copy the local vendor directory
COPY vendor ./vendor

# Copy the rest of the code
COPY . .

# Build using the vendor directory
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o muzej-magriz .

# Stage 2: Runtime
FROM alpine:latest
WORKDIR /root/

# Copy certificates from the builder stage to avoid apk TLS issues
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary and static files
COPY --from=builder /app/muzej-magriz .
COPY --from=builder /app/static ./static

# Environment variables defaults
ENV PORT=8080
EXPOSE 8080

CMD ["./muzej-magriz"]

# Stage 1: Build (Using local vendor folder - OFFLINE build)
FROM golang:1.21-bookworm AS builder
WORKDIR /app

# Copy go.mod and go.sum (just for completeness)
COPY go.mod go.sum ./

# Copy the pre-downloaded vendor directory
COPY vendor ./vendor

# Copy the rest of the code
COPY . .

# Build using the vendor directory (-mod=vendor ensures it doesn't try to download anything)
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o muzej-magriz .

# Stage 2: Runtime (Use Debian slim which has more pre-installed than Alpine)
FROM debian:bookworm-slim
WORKDIR /root/

# Copy binary and static files
COPY --from=builder /app/muzej-magriz .
COPY --from=builder /app/static ./static

# Environment variables defaults
ENV PORT=8080
EXPOSE 8080

CMD ["./muzej-magriz"]

FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod ./
# Copy source code
COPY main.go ./

# Install dependencies and build
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o geoip-api .

# Create minimal final image
FROM alpine:latest

WORKDIR /app

# Install CA certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder stage
COPY --from=builder /app/geoip-api .

# Copy sample config file
COPY config.json .

# Create directory for databases
RUN mkdir -p /app/maxmind_db

# Expose port
EXPOSE 5324

# Ensure the binary is executable
RUN chmod +x /app/geoip-api

# Command to run
CMD ["/app/geoip-api"]

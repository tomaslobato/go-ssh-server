# Use a multi-stage build for smaller final image size
FROM golang:1.22.4-alpine3.19 AS builder

WORKDIR /app

# Copy only the necessary Go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the source code
COPY ./src ./src

# Build the Go application
RUN go build -o /app/main ./src

# Start a new stage for the final minimal image
FROM alpine:latest

# Install necessary packages
RUN apk add --no-cache openssh

# Set working directory
WORKDIR /app

# Copy the compiled binary and SSH key
COPY --from=builder /app/main /app/
COPY ./ssh_server_key /root/.ssh/id_rsa

# Set permissions for SSH key
RUN chmod 400 /root/.ssh/id_rsa

# Expose SSH port
EXPOSE 2222

# Start the SSH server
CMD ["./main"]

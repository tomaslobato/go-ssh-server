# Start with a base golang image for building
FROM golang:1.22.4-alpine3.19 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go app
RUN go build -o main .

# Start a new stage from Alpine Linux
FROM alpine:latest

# Install necessary packages (including OpenSSH for SSH server functionality)
RUN apk add --no-cache openssh

# Copy the built executable from the builder stage
COPY --from=builder /app/main /usr/local/bin/

# Copy your private key (adjust path if necessary)
COPY ./ssh_server_key /root/.ssh/id_rsa

# Set permissions for the private key (recommended: read-only)
RUN chmod 400 /root/.ssh/id_rsa

# Expose SSH port
EXPOSE 2222

# Command to run the executable
CMD ["main"]

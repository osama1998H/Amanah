# Dockerfile for the Amanah Go application

# Use the official Golang image as a base image
FROM golang:1.24 as builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o amanah ./...

# Use a minimal base image for the final stage
FROM alpine:latest

# Set the working directory
WORKDIR /root/

# Copy the built application from the builder stage
COPY --from=builder /app/amanah .

# Expose the application port
EXPOSE 8080

# Command to run the application
CMD ["./amanah"]
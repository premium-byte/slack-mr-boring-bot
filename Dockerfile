# Start from the official Go base image
FROM golang:1.22-alpine AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

RUN ls -la

# Build the Go app
RUN go build -o main ./cmd/mr-boring

# Start a new stage from scratch
FROM alpine:latest

# Set the Current Working Directory inside the container
WORKDIR /root/

# Copy the Pre-built binary file and necessary files from the previous stage
COPY --from=builder /app/configuration.json .
COPY --from=builder /app/current_selection_storage.json .
COPY --from=builder /app/current_support_selection_storage.json .
COPY --from=builder /app/main .


# Set environment variables
ENV APP_NAME="MyApp"
ENV APP_ENV="production"

RUN echo "My environemnt variables"
RUN echo $APP_NAME
RUN echo $APP_ENV

# Command to run the executable
CMD ["./main"]
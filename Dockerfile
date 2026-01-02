# Stage 1: Builder
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum, and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# CGO_ENABLED=0 is important for static binaries in Alpine
ARG GIT_COMMIT=dev
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 go build -ldflags="-X main.commit=${GIT_COMMIT} -X main.buildDate=${BUILD_DATE}" -o /app/knolhash ./cmd/knolhash

# Stage 2: Runner
# Use alpine/git for runtime if go-git needs system git tools,
# otherwise a plain alpine or scratch can be used.
# alpine/git is about 20MB larger than plain alpine, but safer for git operations.
FROM alpine/git AS runner

# Create necessary directories
RUN mkdir -p /app/data /app/data/repos

WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/knolhash .

# Copy config.yaml into the container
COPY config.yaml .

# Expose the port for the web server
EXPOSE 8080

# Define volumes for persistent data (database and cloned repos)
VOLUME /app/data

# Set the entrypoint to run the application
ENTRYPOINT ["./knolhash"]

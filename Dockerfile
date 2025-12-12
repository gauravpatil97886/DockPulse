# ---------- Build Stage ----------
FROM golang:1.25-alpine AS build

# Set working directory inside the container
WORKDIR /app

# Copy Go module files and download dependencies first (better cache)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the project
COPY . .

# Build the DockPulse binary
# (adjust ./cmd/dashboard if your main.go is in a different path)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dockpulse ./cmd/dashboard

# ---------- Runtime Stage ----------
FROM alpine:latest

# Where our app will run inside the container
WORKDIR /app

# Copy the built binary from the build stage
COPY --from=build /app/dockpulse /usr/local/bin/dockpulse

# Ensure it is executable
RUN chmod +x /usr/local/bin/dockpulse

# This is what will run when container starts
ENTRYPOINT ["dockpulse"]

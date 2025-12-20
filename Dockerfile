# Stage 1: Builder
FROM golang:1.25-alpine AS builder

# Install git (required for GOTOOLCHAIN to download newer Go versions)
RUN apk add --no-cache git

# Enable automatic toolchain downloads for newer Go versions required by go.mod
ENV GOTOOLCHAIN=auto

WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o /usr/local/bin/lumescope ./cmd/lumescope

# Stage 2: Runtime (Postgres + App)
FROM postgres:14-alpine

# Set environment defaults (POSTGRES_PASSWORD set in entrypoint.sh to avoid build warning)
ENV PGDATA=/var/lib/postgresql/data
ENV POSTGRES_DB=lumescope
ENV POSTGRES_USER=postgres
ENV PORT=18080
ENV DB_DSN=postgres://postgres:postgres@localhost:5432/lumescope?sslmode=disable

# Set working directory for the app
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /usr/local/bin/lumescope /app/lumescope

# Copy the docs directory for OpenAPI/Swagger
COPY --from=builder /app/docs /app/docs

# Copy the entrypoint script
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Create empty .env file to prevent "file not found" log noise
RUN touch /app/.env

# Expose ports
EXPOSE 18080 5432

# Health check for container orchestration (uses wget available in Alpine)
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:18080/healthz || exit 1

# Set user and entrypoint
USER postgres
ENTRYPOINT ["/app/entrypoint.sh"]
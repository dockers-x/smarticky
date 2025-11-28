FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build with CGO enabled for SQLite
RUN CGO_ENABLED=1 go build -o smarticky ./cmd/server

FROM alpine:latest

# Install ca-certificates and SQLite runtime
RUN apk --no-cache add ca-certificates sqlite-libs

WORKDIR /app

COPY --from=builder /app/smarticky .
COPY --from=builder /app/web ./web

# Create data directory for persistent storage
RUN mkdir -p /data

# Set environment variable for data directory
ENV SMARTICKY_DATA_DIR=/data

EXPOSE 8080

# Create volume for data persistence
VOLUME ["/data"]

CMD ["./smarticky"]

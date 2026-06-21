FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk --no-cache add nodejs npm

# Build arguments for version info
ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG GIT_COMMIT=unknown

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN cd web/app && npm ci && npm run build

# Build with CGO disabled (using pure Go SQLite implementation) and inject version info
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w \
    -X smarticky/internal/version.Version=${VERSION} \
    -X smarticky/internal/version.BuildTime=${BUILD_TIME} \
    -X smarticky/internal/version.GitCommit=${GIT_COMMIT}" \
    -o smarticky ./cmd/server

FROM alpine:latest

# Install ca-certificates, SQLite runtime, and CJK fonts for server-side share image rendering
RUN apk --no-cache add ca-certificates sqlite-libs font-noto-cjk

WORKDIR /app

COPY --from=builder /app/smarticky .
COPY --from=builder /app/web/static ./web/static
COPY --from=builder /app/web/templates ./web/templates

# Create data directory for persistent storage
RUN mkdir -p /data

# Set environment variable for data directory
ENV SMARTICKY_DATA_DIR=/data

EXPOSE 8080

# Create volume for data persistence
VOLUME ["/data"]

CMD ["./smarticky"]

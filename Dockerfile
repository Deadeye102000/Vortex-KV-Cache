# Step 1: Builder Stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy Go module dependency files
COPY go.mod ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Compile static Linux binaries
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /vortex-server ./cmd/vortex-server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /vortex-dashboard ./cmd/vortex-dashboard

# Step 2: Minimal Runtime Stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /vortex-server /app/vortex-server
COPY --from=builder /vortex-dashboard /app/vortex-dashboard

EXPOSE 6379 8080

CMD ["/app/vortex-server"]

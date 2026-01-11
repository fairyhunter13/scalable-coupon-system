# Stage 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy dependency files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary (GOARCH auto-detected from build platform)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -a \
    -installsuffix cgo \
    -o api ./cmd/api

# Stage 2: Minimal runtime
FROM alpine:3.21

RUN apk --no-cache add ca-certificates wget

WORKDIR /app

COPY --from=builder /build/api .

EXPOSE 3000

CMD ["./api"]

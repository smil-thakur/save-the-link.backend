# ---- Build stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Cache module downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/api

# ---- Runtime stage ----
FROM alpine:3.20

# Needed for outbound TLS: the MongoDB Atlas connection and the link-metadata
# fetcher both make HTTPS calls, which require a CA trust store to verify.
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/server .

ENV GIN_MODE=release

# Render (and most hosts) inject $PORT and route to whatever it's set to; gin's
# Run() with no args already reads $PORT itself, falling back to 8080 locally.
EXPOSE 8080

CMD ["./server"]

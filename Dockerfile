# --- Build ---
FROM golang:1.27-alpine AS builder

WORKDIR /app

# Copying dependency files first allows us to use the cache
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server .

# --- Runtime ---
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

# Grab only the compiled binary
COPY --from=builder /app/server /server

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT [ "/server" ]
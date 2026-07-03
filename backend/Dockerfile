# ---- build stage ----
FROM golang:1.22-alpine AS build
WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

# Build a static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

# ---- run stage ----
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=build /out/server /app/server
COPY migrations ./migrations
COPY seed ./seed

ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/server"]

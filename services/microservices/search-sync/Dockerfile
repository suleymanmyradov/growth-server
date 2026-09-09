FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o /search-sync ./services/microservices/search-sync

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /search-sync /usr/local/bin/
ENTRYPOINT ["search-sync"]

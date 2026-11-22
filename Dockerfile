# Stage 1: Build
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o hirelyuk ./cmd/server

# Stage 2: Runtime
FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /app/hirelyuk /usr/local/bin/
COPY --from=builder /app/web/ /app/web/
COPY --from=builder /app/migrations/ /app/migrations/
EXPOSE 8080
CMD ["hirelyuk"]

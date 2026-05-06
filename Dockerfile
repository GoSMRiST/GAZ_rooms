FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o app ./cmd/app
RUN CGO_ENABLED=0 GOOS=linux go build -o migrator ./cmd/migrator

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/app .
COPY --from=builder /app/migrator .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8083

CMD ["./app"]

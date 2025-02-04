FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

COPY cmd/.env /app/.env

RUN CGO_ENABLED=0 GOOS=linux go build -o knv-service ./cmd/main.go

FROM alpine:latest

RUN apk add --no-cache tzdata

RUN ln -s /usr/share/zoneinfo/Asia/Bangkok /etc/localtime

WORKDIR /app

COPY --from=builder /app/knv-service /app/knv-service

COPY --from=builder /app/.env /app/.env

EXPOSE 8080

CMD ["/app/knv-service-KR"]

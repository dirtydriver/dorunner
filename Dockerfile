FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o dorunner .

FROM gcr.io/distroless/base:nonroot

WORKDIR /app

COPY --from=builder /app/dorunner .

CMD ["./dorunner"]
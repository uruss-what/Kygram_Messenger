FROM golang:1.21 AS builder

WORKDIR /Kygram

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /Kygram/server ./cmd

FROM debian:bullseye-slim

WORKDIR /Kygram/
COPY --from=builder /Kygram/server /Kygram/server
RUN chmod +x /Kygram/server

CMD ["/Kygram/server"]
FROM golang:1.25.4-alpine3.21

RUN apk add --no-cache git make curl

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o bin/gateway ./cmd/gateway

EXPOSE 8088

CMD ["./bin/gateway"]

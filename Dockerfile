FROM golang:1.24-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go mod tidy
RUN go build -o main ./cmd/server

# Create a non-root user
RUN adduser -D -g '' appuser

EXPOSE 8080

USER appuser

CMD ["./main"]

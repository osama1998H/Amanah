FROM golang:1.23-alpine
WORKDIR /app
COPY . .
RUN go build -o notification ./cmd
CMD ["/app/notification"]

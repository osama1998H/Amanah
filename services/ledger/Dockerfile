FROM golang:1.23-alpine
WORKDIR /app
COPY . .
RUN go build -o ledger ./cmd
CMD ["/app/ledger"]

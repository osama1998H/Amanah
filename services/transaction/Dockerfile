FROM golang:1.23-alpine
WORKDIR /app
COPY . .
RUN go build -o transaction ./cmd
CMD ["/app/transaction"]


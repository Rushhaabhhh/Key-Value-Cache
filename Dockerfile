FROM golang:1.23-alpine

WORKDIR /app

COPY . .

RUN go build -o key-value-cache .

EXPOSE 7171

CMD ["./key-value-cache"]
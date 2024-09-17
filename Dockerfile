FROM golang:1.23-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o mailshuttle .

FROM alpine:3.20

RUN apk --no-cache add ca-certificates
COPY --from=builder /app/mailshuttle /usr/bin

RUN mkdir -p /etc/mailshuttle
ENV CONFIG_PATH=/etc/mailshuttle/config.yaml

VOLUME ["/var/lib/mailshuttle"]
EXPOSE 2525
ENTRYPOINT ["/usr/bin/mailshuttle"]

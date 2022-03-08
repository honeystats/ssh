FROM golang:1.16-alpine as builder
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY *.go ./
COPY files ./
RUN go build -o /ssh

FROM alpine:latest
WORKDIR /
COPY --from=builder /ssh /ssh
CMD ["/ssh"]

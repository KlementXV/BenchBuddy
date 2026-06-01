# syntax=docker/dockerfile:1.6

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/runner ./cmd/runner

FROM alpine:3.20
RUN apk add --no-cache iperf3 fio ca-certificates
COPY --from=build /out/runner /usr/local/bin/runner
ENTRYPOINT ["/usr/local/bin/runner"]

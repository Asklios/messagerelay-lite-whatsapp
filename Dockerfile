FROM golang:alpine AS builder

RUN apk update && apk add gcc musl-dev

ENV GO111MODULE=on \
    CGO_ENABLED=1 \
    GOOS=linux

WORKDIR /build

COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN go build -a -ldflags '-extldflags "-static"' -o main .

WORKDIR /dist

RUN cp /build/main .

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /dist/main /
ENTRYPOINT ["/main"]

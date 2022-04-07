FROM golang:1.17.6-alpine3.14 as builder

WORKDIR /app
COPY . .
RUN go mod init
RUN go mod tidy -compat=1.17
RUN go mod vendor
RUN go build -o bin/ec2metaproxy


FROM alpine:3.14

MAINTAINER Cory Thomas <cthomas7577@gmail.com>

RUN apk add --no-cache ca-certificates
COPY --from=builder /app/bin/ec2metaproxy /bin/ec2metaproxy

ENTRYPOINT ["/bin/ec2metaproxy"]
FROM alpine

MAINTAINER Cory Thomas <cthomas7577@gmail.com>

RUN apk add --no-cache ca-certificates

COPY ec2metaproxy /bin/ec2metaproxy

ENTRYPOINT ["/bin/ec2metaproxy"]

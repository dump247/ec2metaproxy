FROM busybox:ubuntu-14.04

MAINTAINER Cory Thomas <cthomas7577@gmail.com>

COPY ec2metaproxy /bin/ec2metaproxy

ENTRYPOINT ["/bin/ec2metaproxy"]

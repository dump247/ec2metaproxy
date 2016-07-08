FROM golang:1.6-alpine

MAINTAINER Cory Thomas <cthomas7577@gmail.com>

ENV GOTOOLS=/gotools

ENV PATH $GOTOOLS/bin:$PATH

RUN apk add --no-cache git make                                 \
    &&                                                          \
    GOPATH=$GOTOOLS go get -u github.com/golang/lint/golint     \
    &&                                                          \
    GOPATH=$GOTOOLS go get -u github.com/kardianos/govendor     \
    &&                                                          \
    rm -rf $GOTOOLS/pkg $GOTOOLS/src

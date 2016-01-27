PROJECT_ROOT:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
PROJECT_PACKAGE=github.com/dump247/ec2metaproxy

DOCKER_IMAGE=dump247/ec2metaproxy

GO15VENDOREXPERIMENT=1

SRC_DIRS=${PROJECT_PACKAGE}
CMD_DIRS=${PROJECT_PACKAGE}

.PHONY: clean build test compile fmt docker-image

build: fmt lint test compile

compile:
	go install ${CMD_DIRS}

test:
	go test ${SRC_DIRS}

lint:
	golint ${SRC_DIRS}

fmt:
	go fmt ${SRC_DIRS}

clean:
	go clean -i ${SRC_DIRS}

docker-image: clean
	docker run --rm -e GO15VENDOREXPERIMENT=1 -e GOPATH=/project -v ${GOPATH}:/project -w=/project/src/${PROJECT_PACKAGE} golang:1.5 make
	@cp ${PROJECT_ROOT}/Dockerfile ${GOPATH}/bin/
	docker build -t ${DOCKER_IMAGE} ${GOPATH}/bin/

push-docker-image: docker-image
	docker push ${DOCKER_IMAGE}

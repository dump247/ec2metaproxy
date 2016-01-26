export GOPATH=$(CURDIR)

DOCKER_IMAGE=dump247/ec2metaproxy

SRC_DIRS=github.com/dump247/docker-ec2-metadata
CMD_DIRS=github.com/dump247/docker-ec2-metadata

.PHONY: clean build test compile fmt docker-image

build: fmt test compile

compile:
	go install ${CMD_DIRS}

test:
	go test ${SRC_DIRS}

fmt:
	go fmt ${SRC_DIRS}

clean:
	go clean -i ${SRC_DIRS}
	rm -rf ${GOPATH}/bin
	rm -rf ${GOPATH}/pkg

docker-image: clean
	docker run --rm -v ${GOPATH}:/project -w=/project golang:1.5 make
	@cp Dockerfile bin/
	docker build -t ${DOCKER_IMAGE} bin/

push-docker-image: docker-image
	docker push ${DOCKER_IMAGE}

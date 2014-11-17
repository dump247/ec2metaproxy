export GOPATH=$(CURDIR)

SRC_DIRS=github.com/dump247/docker-ec2-metadata
CMD_DIRS=github.com/dump247/docker-ec2-metadata

.PHONY: clean build test compile fmt

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

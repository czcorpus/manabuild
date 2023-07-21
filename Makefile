VERSION=`git describe --tags --always`
BUILD=`date +%FT%T%z`
HASH=`git rev-parse --short HEAD`


LDFLAGS=-ldflags "-w -s -X main.version=${VERSION} -X main.buildDate=${BUILD} -X main.gitCommit=${HASH}"

all: test build

build:
	go build -o manabuild ${LDFLAGS}

install:
	go install -o manabuild ${LDFLAGS}

clean:
	rm manabuild

test:
	go test ./...

rtest:
	go test -race ./...

.PHONY: clean install test
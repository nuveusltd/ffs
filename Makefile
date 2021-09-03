VERSION=1.0
BUILD_NO=$(shell cat ./.buildno)
BUILD=`git rev-parse HEAD`
LDFLAGS=-ldflags "-X main.BuildNumber=$(BUILD_NO) -X main.Version=$(VERSION) -X main.BuildHash=$(BUILD)"
XC_OS="linux darwin"
XC_ARCH="amd64"
XC_PARALLEL="3"
BIN="./bin"
SRC=$(shell find . -name "*.go")

ifeq (, $(shell which gox))
$(warning "could not find gox in $(PATH), run: go get github.com/mitchellh/gox")
endif

.PHONY: all build

default: all

all: build

build:
	GO111MODULE=on gox $(LDFLAGS) \
		-os=$(XC_OS) \
		-arch=$(XC_ARCH) \
		-parallel=$(XC_PARALLEL) \
		-output=$(BIN)/ffs_{{.OS}} \
		&& let "BUILD_NO=$(BUILD_NO)+1" && echo $$BUILD_NO > ./.buildno ;

run : build
		chmod +x bin/ffs_darwin
		bin/ffs_darwin --mountpoint ~/tmp/testFolder/mp --source ~/tmp/testFolder/sourcea --source ~/tmp/testFolder/sourceb --checksumdir ~/tmp/testFolder/sourcec
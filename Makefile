VERSION=$(shell git describe --tags)

export GO15VENDOREXPERIMENT=1

NAME=hubbard
DESCRIPTION="hubbard is a proxy for seamlessly working with authenticated GitHub resources"

CCOS=windows darwin linux
CCARCH=amd64
CCOUTPUT="pkg/{{.OS}}-{{.Arch}}/$(NAME)"

GO_VERSION=$(shell go version)

# Get the git commit
SHA=$(shell git rev-parse --short HEAD)
BUILD_COUNT=$(shell git rev-list --count HEAD)

BUILD_TAG="${BUILD_COUNT}.${SHA}"

UNAME := $(shell uname -s)

.PHONY: default
default: banner clean gox

.PHONY: banner
banner:
	@echo "Go Version:     ${GO_VERSION}"
	@echo "Go Path:        ${GOPATH}"
	@echo "Binary Name:    $(NAME)"
	@echo "Binary Version: $(VERSION)"


.PHONY: vendor
vendor:
	@printf "\n==> Running Glide install\n"
	curl https://glide.sh/get | sh
	glide install

.PHONY: gox
gox: vendor
	@printf "\n==> Using Gox to cross-compile $(NAME)\n"
	go get github.com/mitchellh/gox
	@gox -ldflags="-X github.build.ge.com/secc/hubbard/cmd.Version=${VERSION}" \
	     -os="$(CCOS)" \
			 -arch="$(CCARCH)" \
			 -output=$(CCOUTPUT)

.PHONY: package
package: SHELL:=/bin/bash
package:
	@printf "\n==> Creating distributable packages\n"
	@set -exv
	@mkdir -p release/
	@for os in $(CCOS);                                          				\
	 do                                                                 \
		  for arch in $(CCARCH);                                          \
		  do                                                              \
		    cd "pkg/$$os-$$arch/" || exit;                                \
	      filename=hubbard-$$os-$$arch-$(VERSION).tar.gz;               \
	      echo Creating: release/$$filename;                            \
	      tar -zcvf ../../release/$$filename hubbard* > /dev/null 2>&1; \
		    cd ../../ || exit;                                            \
		  done                                                            \
	 done
	@printf "\n==> Done Cross Compiling $(NAME)\n"

.PHONY: clean
clean:
	@printf "\n==> Cleaning\n"
	rm -rf release/
	rm -rf bin/
	rm -rf pkg/

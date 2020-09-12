REGISTRY ?= "ghcr.io/uesyn/pod-limit-oom-recorder"
TAG ?= latest
OUTDIR ?= bin
REPO ?= $(shell git remote get-url origin)

ifeq ($(origin VERSION), undefined)
  VERSION := git-$(shell git rev-parse --short HEAD)
endif

all: build 

.PHONY: build push

build:
	GOOS=linux go build \
	-ldflags "-w -X main.version=$(VERSION) -X main.gitRepo=$(REPO)" \
	-o $(OUTDIR)/pod-limit-oom-recorder

image:
	docker build -t "$(REGISTRY):$(TAG)" .

push: image
	docker push "$(REGISTRY):$(TAG)"

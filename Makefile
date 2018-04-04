##
## Building
##
build:
	./build.sh

linux_binary:
	./build.sh linux/amd64

ios_framework:
	CGO_CFLAGS_ALLOW='-fmodules|-fblocks' gomobile bind -target=ios/arm64 github.com/textileio/textile-go/mobile

android_framework:
	gomobile bind -target=android -o textilego.aar github.com/textileio/textile-go/mobile

##
## Docker
##
DOCKER_PROFILE ?= openbazaar
DOCKER_SERVER_VERSION ?= $(shell git describe --tags --abbrev=0)
DOCKER_SERVER_IMAGE_NAME ?= $(DOCKER_PROFILE)/server:$(DOCKER_SERVER_VERSION)
DOCKER_DUMMY_IMAGE_NAME ?= $(DOCKER_PROFILE)/server_dummy:$(DOCKER_SERVER_VERSION)

docker:
	docker build -t $(DOCKER_SERVER_IMAGE_NAME) .

push_docker:
	docker push $(DOCKER_SERVER_IMAGE_NAME)

deploy_docker: docker push_docker

dummy_docker:
	docker build -t $(DOCKER_DUMMY_IMAGE_NAME) -f Dockerfile.dummy .

push_dummy_docker:
	docker push $(DOCKER_DUMMY_IMAGE_NAME)

deploy_dummy_docker: dummy_docker push_dummy_docker

##
## Cleanup
##
clean_build:
	rm -f ./dist/*

clean_docker:
	docker rmi -f $(DOCKER_SERVER_IMAGE_NAME) $(DOCKER_DUMMY_IMAGE_NAME) || true

clean: clean_build clean_docker

build:
	./build.sh

linux_binary:
	./build.sh linux/amd64

build_desktop:
	$(MAKE) -C ./desktop build

ios_framework:
	CGO_CFLAGS_ALLOW='-fmodules|-fblocks' gomobile bind -target=ios/arm64 github.com/textileio/textile-go/mobile github.com/textileio/textile-go/net
	# cp -r Mobile.framework ~/github/textileio/textile-mobile/ios/

android_framework:
	gomobile bind -target=android -o textilego.aar github.com/textileio/textile-go/mobile github.com/textileio/textile-go/net
	# cp -r textilego.aar ~/github/textileio/textile-mobile/android/textilego/

clean_build:
	rm -rf dist && rm -f Mobile.framework

clean: clean_build

docker_build:
	docker build -t circleci:1.10 .

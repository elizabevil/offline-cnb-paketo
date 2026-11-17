all: build

##@ Build Binary
.PHONY: build
build:  ## 在蓝盾构建, 由于蓝盾插件不支持 golang 1.18, 所以实际上只执行了 cp 命令
	mkdir -p ../../bin
	cp bin/$$(go env GOOS) ../../bin/app
	chmod +x  ../../bin/app


.PHONY: build-cli
build-cli: ## 构建 Cli
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o bin/bksmartpacker-linux-ts4 -ldflags '-extldflags "-static" -w -s' ./cmd/bksmartpacker
	CGO_ENABLED=0 GOARCH=amd64 GOOS=darwin go build -o bin/bksmartpacker-darwin-ts4 -ldflags '-extldflags "-static" -w -s' ./cmd/bksmartpacker
	CGO_ENABLED=0 GOARCH=amd64 GOOS=windows go build -o bin/bksmartpacker-windows-ts4 -ldflags '-extldflags "-static" -w -s' ./cmd/bksmartpacker


.PHONY: build-cli-arm64
build-cli-arm64: ## 构建 arm64 架构 cli
	CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go build -o bin/bksmartpacker-linux-ts4-arm64 -ldflags '-extldflags "-static" -w -s' ./cmd/bksmartpacker


##@ Misc
.PHONY: help
help:  ## Show this message
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

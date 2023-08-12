# Go parameters
GOCMD?=go
GOBUILD?=$(GOCMD) build
GOCLEAN?=$(GOCMD) clean
GOTEST?=$(GOCMD) test
GOGET?=$(GOCMD) get
GOBIN?=$(GOPATH)/bin

OUT?=./out
IMAGE_REPO?=ghcr.io/aeraki-mesh
IMAGE_NAME?=consul2istio
IMAGE_TAG?=latest
IMAGE?=$(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG)
MAIN_PATH=./cmd/consul2istio/main.go
IMAGE_OS?=linux
IMAGE_ARCH?=amd64
IMAGE_DOCKERFILE_PATH?=docker/Dockerfile

build: test
	CGO_ENABLED=0 GOOS=$(IMAGE_OS) GOARCH=$(IMAGE_ARCH) $(GOBUILD) -o $(OUT)/$(IMAGE_ARCH)/$(IMAGE_OS)/$(IMAGE_NAME) $(MAIN_PATH)
docker-build: build
	docker build --build-arg CONSUL2ISTIO_BIN_DIR=${OUT} --build-arg ARCH=${IMAGE_ARCH} --build-arg OS=${IMAGE_OS} \
	--no-cache --platform=${IMAGE_OS}/${IMAGE_ARCH} -t ${IMAGE} -f ${IMAGE_DOCKERFILE_PATH} .
docker-push: docker-build
	docker push $(IMAGE)
style-check:
	gofmt -l -d ./
	goimports -l -d ./
lint:
	golangci-lint  run -v
test:
	go test --race ./...
clean:
	rm -rf $(OUT)

.PHONY: build docker-build docker-push clean

APP_NAME := rclone-manager
IMAGE ?= ghcr.io/kevindiu/rclone-manager
APP_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
RCLONE_VERSION ?= 1.76.0

.PHONY: build-web lint-web build-go build vet test check dev docker-build docker-buildx docker-push clean

lint-web:
	cd web && npm run lint

build-web:
	cd web && npm ci && npm run build

build-go:
	go build -trimpath -ldflags="-s -w -X main.version=$(APP_VERSION)" -o bin/$(APP_NAME) ./cmd/rclone-manager

build: build-web build-go

vet:
	go vet ./...

test:
	go test -race -v ./...

check: lint-web build-web vet test

dev:
	go run ./cmd/rclone-manager

docker-build:
	docker build --build-arg RCLONE_VERSION=$(RCLONE_VERSION) --build-arg APP_VERSION=$(APP_VERSION) -t $(APP_NAME):$(APP_VERSION) .

docker-buildx:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg RCLONE_VERSION=$(RCLONE_VERSION) \
		--build-arg APP_VERSION=$(APP_VERSION) \
		-t $(APP_NAME):$(APP_VERSION) .

docker-push:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg RCLONE_VERSION=$(RCLONE_VERSION) \
		--build-arg APP_VERSION=$(APP_VERSION) \
		-t $(IMAGE):$(APP_VERSION) \
		--push .

clean:
	rm -rf bin dist

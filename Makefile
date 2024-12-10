IMAGE ?= Ladicle/keda-manual-scaler
TAG ?= latest

all: test build-image load-image deploy

.PHONY: test
test: go-mod-tidy go-vet go-test go-test-race

.PHONY: go-mod-tidy
go-mod-tidy:
	@go mod tidy

.PHONY: go-vet
go-vet:
	@go vet ./...

.PHONY: go-test
go-test:
	@go test ./...

.PHONY: go-test-race
go-test-race:
	@go test -race ./...

.PHONY: build-image
build-image:
	@docker build -t $(IMAGE):$(TAG) .

.PHONY: load-image
load-image:
	@kind load docker-image $(IMAGE):$(TAG)

DEPLOY_NAME ?= test
DEPLOY_NS ?= default

.PHONY: deploy
deploy:
	@helm upgrade --create-namespace -n $(DEPLOY_NS) --install $(DEPLOY_NAME) chart/

DIR := ${CURDIR}

CONTROLLER_GEN=GO111MODULE=on GOFLAGS=-mod=vendor go run ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen
CRD_OPTIONS ?= "crd:trivialVersions=true"

GO_GCFLAGS ?= -gcflags=all='-N -l'
GO=GO111MODULE=on GOFLAGS=-mod=vendor go
GO_BUILD_RECIPE=CGO_ENABLED=0 $(GO) build $(GO_GCFLAGS)

all: build

.PHONY.: build
build: hypershift-lite

.PHONY.: hypershift-lite
hypershift-lite:
	$(GO_BUILD_RECIPE) -o bin/hypershift-lite ./cmd/hypershift-lite

.PHONY.: api
api:
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./pkg/api/..."
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./thirdparty/etcd/..."
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./pkg/api/..." output:crd:artifacts:config=config/crd
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./thirdparty/etcd/..." output:crd:artifacts:config=config/crd

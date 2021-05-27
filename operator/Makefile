
# Image URL to use all building/pushing image targets
IMG_REPO ?= gcr.io/shopify-docker-images/cloud/production/
IMG_SHA ?= latest
INIT_IMG ?= $(join ${IMG_REPO}, kubectl-trace-init):${IMG_SHA}
RUNNER_IMG ?= $(join ${IMG_REPO}, kubectl-trace-runner):${IMG_SHA}
OPERATOR_IMG ?= $(join ${IMG_REPO}, kubectl-trace-operator):${IMG_SHA}
DEPLOY_CONTEXT ?= $(shell kubectl config current-context 2>/dev/null)
CRD_OPTIONS ?= "crd:crdVersions=v1"

DRY_RUN ?=            # set DRY_RUN=1 to output, but not apply, kustomize rendered output
GENERATE_MANIFESTS ?= # set GENERATE_MANIFESTS=1 to force rebuilding manifests

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

kubectl-trace-controller = _output/bin/kubectl-trace-controller

all: clean ${kubectl-trace-controller}

clean:
	$(RM) -R _output

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build kubectl-trace-controller binary
${kubectl-trace-controller}: generate #fmt vet
	CGO_ENABLED=0 go build -o $@ ./cmd/kubectl-trace-controller

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./cmd/kubectl-trace-controller/main.go

# Install CRDs into a cluster
install: $(if $(GENERATE_MANIFESTS), manifests)
	kubectl kustomize config/global $(if $(DRY_RUN), , | kubectl apply -f - --context $(DEPLOY_CONTEXT))

# Uninstall CRDs from a cluster
uninstall: $(if $(GENERATE_MANIFESTS), manifests)
	kubectl kustomize config/global | kubectl delete -f - --context $(DEPLOY_CONTEXT)

# Install CRDs into a cluster
client-install: $(if $(GENERATE_MANIFESTS), manifests)
	kubectl kustomize config/client $(if $(DRY_RUN), , | kubectl apply -f - --context $(DEPLOY_CONTEXT))

# Uninstall CRDs from a cluster
client-uninstall: $(if $(GENERATE_MANIFESTS), manifests)
	kubectl kustomize config/client | kubectl delete -f - --context $(DEPLOY_CONTEXT)

# Install RBAC rules in the cluster
rbac-install: $(if $(GENERATE_MANIFESTS), manifests)
	kubectl kustomize config/rbac $(if $(DRY_RUN), , | kubectl apply -f - --context $(DEPLOY_CONTEXT))

# Uninstall RBAC rules in the cluster
rbac-uninstall: $(if $(GENERATE_MANIFESTS), manifests)
	kubectl kustomize config/rbac | kubectl delete -f - --context $(DEPLOY_CONTEXT)

# Install default CRs into a cluster
defaults-install: $(if $(GENERATE_MANIFESTS), manifests)
	kubectl kustomize config/defaults $(if $(DRY_RUN), , | kubectl apply -f - --context $(DEPLOY_CONTEXT))

# Uninstall default CRs from a cluster
defaults-uninstall: $(if $(GENERATE_MANIFESTS), manifests)
	kubectl kustomize config/defaults | kubectl delete -f - --context $(DEPLOY_CONTEXT)

set-revisions:
	cd config/preloader && kustomize edit set image preloader=${INIT_IMG}
	cd config/preloader && kustomize edit set image tracerunner=${RUNNER_IMG}
	cd config/manager && kustomize edit set image controller=${OPERATOR_IMG}

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: $(if $(GENERATE_MANIFESTS), manifests) set-revisions
	kustomize build config/base $(if $(DRY_RUN), , | kubectl apply -f - --context $(DEPLOY_CONTEXT))

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object paths="./..."

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

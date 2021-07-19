#Use bash as shell
SHELL = /bin/bash

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Image URL to use all building/pushing image targets
DEFAULT_AUTHORINO_IMAGE = quay.io/3scale/authorino:latest
AUTHORINO_IMAGE ?= $(DEFAULT_AUTHORINO_IMAGE)
# The Kubernetes namespace where to deploy the Authorino instance.
AUTHORINO_NAMESPACE ?= authorino
# Flavour of the Authorino deployment – Options: 'namespaced' (default), 'cluster-wide'
AUTHORINO_DEPLOYMENT ?= namespaced
# Number of Authorino replicas
AUTHORINO_REPLICAS ?= 1

all: manager

# Run tests
ENVTEST_ASSETS_DIR = $(shell pwd)/testbin
test: generate fmt vet manifests
	mkdir -p $(ENVTEST_ASSETS_DIR)
	test -f $(ENVTEST_ASSETS_DIR)/setup-envtest.sh || curl -sSLo $(ENVTEST_ASSETS_DIR)/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.6.3/hack/setup-envtest.sh
	source $(ENVTEST_ASSETS_DIR)/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./... -coverprofile cover.out

# Show test coverage
cover:
	go tool cover -html=cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/authorino main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build install | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build install | kubectl delete -f -

# Requests TLS certificates for services if cert-manager.io is installed, the secret is not already present and TLS is enabled
.PHONY: certs
TLS_CERT_SECRET_CHECK = $(shell kubectl -n $(AUTHORINO_NAMESPACE) get secret/authorino-oidc-server-cert 2>/dev/null)
CERT_MANAGER_CHECK = $(shell kubectl get crds/issuers.cert-manager.io 2>/dev/null)
certs:
ifeq (,$(findstring -notls,$(AUTHORINO_DEPLOYMENT)))
ifeq (,$(TLS_CERT_SECRET_CHECK))
ifneq (, $(CERT_MANAGER_CHECK))
	cd deploy/base/certmanager && $(KUSTOMIZE) edit set namespace $(AUTHORINO_NAMESPACE)
	$(KUSTOMIZE) build deploy/base/certmanager | kubectl -n $(AUTHORINO_NAMESPACE) apply -f -
	cd deploy/base/certmanager && $(KUSTOMIZE) edit set namespace authorino
else
	echo "cert-manager not installed."
endif
else
	echo "tls cert secret found."
endif
else
	echo "tls disabled."
endif

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	$(MAKE) certs AUTHORINO_NAMESPACE=$(AUTHORINO_NAMESPACE) AUTHORINO_DEPLOYMENT=$(AUTHORINO_DEPLOYMENT)
	cd deploy/base && $(KUSTOMIZE) edit set image authorino=$(AUTHORINO_IMAGE) && $(KUSTOMIZE) edit set namespace $(AUTHORINO_NAMESPACE) && $(KUSTOMIZE) edit set replicas authorino-controller-manager=$(AUTHORINO_REPLICAS)
	cd deploy/overlays/$(AUTHORINO_DEPLOYMENT) && $(KUSTOMIZE) edit set namespace $(AUTHORINO_NAMESPACE)
	$(KUSTOMIZE) build deploy/overlays/$(AUTHORINO_DEPLOYMENT) | kubectl -n $(AUTHORINO_NAMESPACE) apply -f -
# rollback kustomize edit
	cd deploy/base && $(KUSTOMIZE) edit set image authorino=$(DEFAULT_AUTHORINO_IMAGE) && $(KUSTOMIZE) edit set namespace authorino && $(KUSTOMIZE) edit set replicas authorino-controller-manager=1
	cd deploy/overlays/$(AUTHORINO_DEPLOYMENT) && $(KUSTOMIZE) edit set namespace authorino

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) crd:trivialVersions=true,crdVersions=v1 rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=install/crd  output:rbac:artifacts:config=install/rbac

# Download vendor dependencies
.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: vendor controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: vendor
	docker build . -t ${AUTHORINO_IMAGE}

# Push the docker image
docker-push:
	docker push ${AUTHORINO_IMAGE}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

kind:
ifeq (, $(shell which kind))
	@{ \
	set -e ;\
	KIND_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KIND_GEN_TMP_DIR ;\
	go mod init tmp ;\
	GO111MODULE="on" go get sigs.k8s.io/kind@v0.9.0 ;\
	rm -rf $$KIND_GEN_TMP_DIR ;\
	}
KIND=$(GOBIN)/kind
else
KIND=$(shell which kind)
endif

# Prints relevant environment variables
.PHONY: envs
envs:
	@{ \
	echo "CONTROLLER_GEN=$(CONTROLLER_GEN)"; \
	echo "KUSTOMIZE=$(KUSTOMIZE)"; \
	echo "AUTHORINO_IMAGE=$(AUTHORINO_IMAGE)"; \
	echo "AUTHORINO_NAMESPACE=$(AUTHORINO_NAMESPACE)"; \
	echo "AUTHORINO_DEPLOYMENT=$(AUTHORINO_DEPLOYMENT)"; \
	echo "AUTHORINO_REPLICAS=$(AUTHORINO_REPLICAS)"; \
	}

# Creates a namespace where to deploy Authorino
.PHONY: namespace
namespace:
	kubectl create namespace $(AUTHORINO_NAMESPACE)

# Deploys the examples user apps: Talker API and Envoy proxy, and optionally Keycloak and Dex
.PHONY: example-apps
NAMESPACE ?= $(AUTHORINO_NAMESPACE)
DEPLOY_KEYCLOAK ?= $(DEPLOY_IDPS)
DEPLOY_DEX ?= $(DEPLOY_IDPS)
example-apps:
	kubectl -n $(NAMESPACE) apply -f examples/talker-api/talker-api-deploy.yaml
	kubectl -n $(NAMESPACE) apply -f examples/envoy/envoy-deploy.yaml
ifneq (, $(DEPLOY_KEYCLOAK))
	kubectl -n $(NAMESPACE) apply -f examples/keycloak/keycloak-deploy.yaml
endif
ifneq (, $(DEPLOY_DEX))
	kubectl -n $(NAMESPACE) apply -f examples/dex/dex-deploy.yaml
endif

# Install CertManager to the Kubernetes cluster
.PHONY: cert-manager
cert-manager:
ifeq (,$(findstring -notls,$(AUTHORINO_DEPLOYMENT)))
	kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.4.0/cert-manager.yaml
	kubectl delete mutatingwebhookconfiguration.admissionregistration.k8s.io/cert-manager-webhook
	kubectl delete validatingwebhookconfigurations.admissionregistration.k8s.io/cert-manager-webhook
	kubectl -n cert-manager wait --timeout=300s --for=condition=Available deployments --all
endif

# Targets with the 'local-' prefix, for trying Authorino in a local cluster spawned with Kind

KIND_CLUSTER_NAME ?= authorino

# Start a local Kubernetes cluster using Kind
.PHONY: local-cluster-up
local-cluster-up: kind
	kind create cluster --name $(KIND_CLUSTER_NAME) --config ./utils/kind-cluster.yaml

# Builds an image locally and pushes it to the registry of the Kind-started local Kubernetes cluster
.PHONY: local-build-and-push
local-build-and-push:
ifneq (1, $(SKIP_LOCAL_BUILD))
	$(eval AUTHORINO_IMAGE = authorino:local)
	$(MAKE) docker-build AUTHORINO_IMAGE=$(AUTHORINO_IMAGE)
	$(MAKE) local-push AUTHORINO_IMAGE=$(AUTHORINO_IMAGE)
endif

# Pushes the Authorino image to the registry of the Kind-started local Kubernetes cluster
.PHONY: local-push
local-push: kind
	kind load docker-image $(AUTHORINO_IMAGE) --name $(KIND_CLUSTER_NAME)

# Deploys Authorino and sets imagePullPolicy to 'IfNotPresent' (so it doesn't try to pull the image which may have just been pushed into the server registry)
.PHONY: local-deploy
local-deploy: deploy
	kubectl -n $(AUTHORINO_NAMESPACE) patch deployment authorino-controller-manager -p '{"spec": {"template": {"spec":{"containers":[{"name": "manager", "imagePullPolicy":"IfNotPresent"}]}}}}'

# Set up a test/dev local Kubernetes server loaded up with a freshly built Authorino image plus dependencies
.PHONY: local-setup
local-setup: local-cluster-up local-build-and-push cert-manager install namespace local-deploy example-apps
	kubectl -n $(AUTHORINO_NAMESPACE) wait --timeout=300s --for=condition=Available deployments --all
	@{ \
	echo "Now you can export the envoy service by doing:"; \
	echo "kubectl port-forward --namespace $(NAMESPACE) deployment/envoy 8000:8000"; \
	echo "After that, you can curl -H \"Host: myhost.com\" localhost:8000"; \
	}

# Rebuild and push the docker image and redeploy Authorino to the local k8s cluster
.PHONY: local-rollout
local-rollout: local-build-and-push
	kubectl -n $(AUTHORINO_NAMESPACE) rollout restart deployment.apps/authorino-controller-manager

# Deletes the local Kubernetes cluster started using Kind
.PHONY: local-cleanup
local-cleanup: kind
	kind delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: up down logs build test restart

up:
	podman-compose up -d --build

down:
	podman-compose down

logs:
	podman-compose logs -f

restart:
	podman-compose restart

test:
	go test -v ./...

build-local:
	go build -o escocar-server ./cmd/server

REGISTRY ?= us-central1-docker.pkg.dev
IMAGE_NAME ?= $(REGISTRY)/escocar-web/escocar-repo/escocar-web:latest
VM_NAME ?= instance-20261025-153912
VM_ZONE ?= us-central1-a
PROJECT ?= escocar-web

build-image:
	podman build -t $(IMAGE_NAME) -f Containerfile .

auth-registry:
	gcloud auth print-access-token | podman login -u oauth2accesstoken --password-stdin $(REGISTRY)

push-image: build-image auth-registry
	podman push $(IMAGE_NAME)

deploy: push-image
	gcloud compute scp compose.yaml "$(VM_NAME):~/escocar-web/compose.yaml" --zone "$(VM_ZONE)" --project "$(PROJECT)"
	gcloud compute ssh --zone "$(VM_ZONE)" "$(VM_NAME)" --project "$(PROJECT)" -- \
		"gcloud auth configure-docker $(REGISTRY) --quiet && \
		podman pull $(IMAGE_NAME) && \
		cd ~/escocar-web && podman-compose down && podman-compose up -d"

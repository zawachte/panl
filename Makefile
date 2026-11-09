.PHONY: build test browser image chart
build:
	cd web && npm ci && npm run build
	go build -trimpath -o panl .
test:
	go test -race ./...
	go vet ./...
	cd web && npm test
browser:
	go build -o panl .
	cd web && npx playwright test
image:
	docker build -t panl:0.1.0 .
chart:
	helm lint deploy/helm/panl
	helm package deploy/helm/panl --destination artifacts

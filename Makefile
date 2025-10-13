
.PHONY: build
build:
	go build -o artifacts/svc ./cmd/cbom-repository

.PHONY: unit_test
unit_test:
	go test -parallel 6 -race -count=1 -coverprofile=ut_coverage.out -v ./...

.PHONY: mockgen
mockgen:
	go tool mockgen -source=internal/store/store.go -destination internal/store/mock/mock.go -package=mock

.PHONY: docker-compose-up 
docker-compose-up:
	docker compose up -d --force-recreate --always-recreate-deps --renew-anon-volumes

.PHONY: docker-compose-down
docker-compose-down:
	docker compose down --volumes --remove-orphans

run-api:
	cd backend && go run ./cmd/api

test:
	cd backend && go test ./...

build-mobile:
	cd mobile && npm run typecheck

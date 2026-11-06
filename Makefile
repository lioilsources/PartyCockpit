# PartyCockpit — one Go binary with the Svelte app embedded.
BIN := bin/partycockpit

.PHONY: all web build run dev test check docker styles dist shellcheck

all: build

# both UIs: the Svelte app at /, the React Cockpit (D21) at /react/
web:
	cd frontend && npm ci --no-audit --no-fund && npm run build
	cd frontend-react && npm ci --no-audit --no-fund && npm run build
	rm -rf backend/internal/web/dist/assets backend/internal/web/dist/react
	mkdir -p backend/internal/web/dist/react
	cp -R frontend/dist/. backend/internal/web/dist/
	cp -R frontend-react/dist/. backend/internal/web/dist/react/

build: web
	cd backend && CGO_ENABLED=0 go build -trimpath -o ../$(BIN) ./cmd/partycockpit

run: build
	set -a; [ -f .env ] && . ./.env; set +a; ./$(BIN)

# backend on :8098 + vite on :5173 (Svelte) and :5174/react/ (React Cockpit); both proxy /api, /ws, /files
dev:
	(cd backend && set -a && ([ -f ../.env ] && . ../.env || true) && set +a && go run ./cmd/partycockpit) & \
	(cd frontend-react && npm run dev) & \
	cd frontend && npm run dev

test:
	cd backend && go vet ./... && go test ./...

check: test shellcheck
	cd frontend && npm run check && npm run build
	cd frontend-react && npm run check && npm run build

# party box scripts (deploy/box); skipped when shellcheck is not installed
shellcheck:
	@if command -v shellcheck >/dev/null; then shellcheck deploy/box/*.sh && echo "shellcheck ok"; \
	else echo "shellcheck not installed, skipping deploy/box checks"; fi

# Release packages: party box (D20) dist/partycockpit-linux-{arm64,amd64}.tar.gz, laptop dist/partycockpit-windows-amd64.zip
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
ARCHES := arm64 amd64
dist: web
	rm -rf dist && mkdir -p dist
	@for arch in $(ARCHES); do \
		pkg=dist/partycockpit-$(VERSION)-linux-$$arch; \
		mkdir -p $$pkg/bin $$pkg/assets $$pkg/deploy; \
		(cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -trimpath \
			-ldflags "-s -w -X main.version=$(VERSION)" -o ../$$pkg/bin/partycockpit ./cmd/partycockpit) || exit 1; \
		cp -R assets/drivers $$pkg/assets/; \
		cp -R deploy/box $$pkg/deploy/; \
		cp .env.example deploy/laptop/README.txt $$pkg/; \
		echo $(VERSION) > $$pkg/VERSION; \
		COPYFILE_DISABLE=1 tar --no-xattrs -czf dist/partycockpit-linux-$$arch.tar.gz -C dist partycockpit-$(VERSION)-linux-$$arch; \
		rm -rf $$pkg; \
		echo "dist/partycockpit-linux-$$arch.tar.gz ($(VERSION))"; \
	done
	@pkg=dist/partycockpit-$(VERSION)-windows-amd64; \
	mkdir -p $$pkg/assets && \
	(cd backend && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath \
		-ldflags "-s -w -X main.version=$(VERSION)" -o ../$$pkg/partycockpit.exe ./cmd/partycockpit) && \
	cp -R assets/drivers $$pkg/assets/ && \
	cp .env.example $$pkg/env.example && \
	cp deploy/laptop/README.txt $$pkg/ && \
	echo $(VERSION) > $$pkg/VERSION && \
	(cd dist && rm -f partycockpit-windows-amd64.zip && COPYFILE_DISABLE=1 zip -qrX partycockpit-windows-amd64.zip partycockpit-$(VERSION)-windows-amd64) && \
	rm -rf $$pkg && echo "dist/partycockpit-windows-amd64.zip ($(VERSION))"

# Regenerate the style catalog from a Restyler checkout (RESTYLER_DIR=../Restyler)
styles:
	node scripts/sync-restyler-styles.mjs $${RESTYLER_DIR:-../Restyler}/packages/tsumiki_shared/lib/catalogs/restyle_styles.dart backend/internal/providers/restyle_styles.json

docker:
	docker build -t partycockpit -f deploy/Dockerfile .

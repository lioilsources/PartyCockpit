# PartyCockpit — one Go binary with the Svelte app embedded.
BIN := bin/partycockpit

.PHONY: all web build run dev test check docker styles

all: build

web:
	cd frontend && npm ci --no-audit --no-fund && npm run build
	rm -rf backend/internal/web/dist/assets
	mkdir -p backend/internal/web/dist
	cp -R frontend/dist/. backend/internal/web/dist/

build: web
	cd backend && CGO_ENABLED=0 go build -trimpath -o ../$(BIN) ./cmd/partycockpit

run: build
	set -a; [ -f .env ] && . ./.env; set +a; ./$(BIN)

# backend on :8098 + vite dev server on :5173 (proxies /api, /ws, /files)
dev:
	(cd backend && set -a && ([ -f ../.env ] && . ../.env || true) && set +a && go run ./cmd/partycockpit) & \
	cd frontend && npm run dev

test:
	cd backend && go vet ./... && go test ./...

check: test
	cd frontend && npm run check && npm run build

# Regenerate the style catalog from a Restyler checkout (RESTYLER_DIR=../Restyler)
styles:
	node scripts/sync-restyler-styles.mjs $${RESTYLER_DIR:-../Restyler}/packages/tsumiki_shared/lib/catalogs/restyle_styles.dart backend/internal/providers/restyle_styles.json

docker:
	docker build -t partycockpit -f deploy/Dockerfile .

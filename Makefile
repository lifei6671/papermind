.PHONY: help frontend-build frontend-dev backend-build backend-dev web-build web-dev server-build server-dev

GO_TAGS ?= json1
BACKEND_BIN ?= bin/papermind.exe
BACKEND_DEV_ENV = set "PAPERMIND_DATABASE_DSN=file:data/sqlite/papermind.db?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000" && \
	set "PAPERMIND_STORAGE_TEMP_DIR=data/tmp" && \
	set "PAPERMIND_STORAGE_IMPORT_DIR=data/imports" && \
	set "PAPERMIND_STORAGE_EXPORT_DIR=data/exports"

help:
	@echo "PaperMind Make targets:"
	@echo "  make frontend-build  打包前端"
	@echo "  make frontend-dev    启动前端开发服务"
	@echo "  make backend-build   编译后端"
	@echo "  make backend-dev     启动后端服务"
	@echo ""
	@echo "Aliases:"
	@echo "  make web-build       = frontend-build"
	@echo "  make web-dev         = frontend-dev"
	@echo "  make server-build    = backend-build"
	@echo "  make server-dev      = backend-dev"

frontend-build:
	cd web && pnpm build

frontend-dev:
	cd web && pnpm dev

backend-build:
	cd server && if not exist bin mkdir bin && go build -tags $(GO_TAGS) -o $(BACKEND_BIN) ./cmd/papermind

backend-dev:
	cd server && $(BACKEND_DEV_ENV) && go run -tags $(GO_TAGS) ./cmd/papermind

web-build: frontend-build

web-dev: frontend-dev

server-build: backend-build

server-dev: backend-dev

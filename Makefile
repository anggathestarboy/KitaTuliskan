# make dev   → MySQL check + Go server (auto-reload) + Vite (TS + Tailwind, HMR) + Astro dev, semua dalam satu terminal
# make build → binary Go + Vite build + Astro static build
SHELL := /bin/bash
.PHONY: dev stop build setup install fresh server css site test clean

# make install NAME=Admin EMAIL=admin@example.com PASSWORD=secret1234  → padanan `php artisan draft:setup`
NAME ?= Admin
EMAIL ?= admin@example.com
PASSWORD ?= change-me-1234
SITE_NAME ?= My Site
SITE_HANDLE ?= indonesia
fresh: setup ## drop database lalu install ulang (dev only)
	cd server && go run . fresh -name "$(NAME)" -email "$(EMAIL)" -password "$(PASSWORD)" -site-name "$(SITE_NAME)" -site-handle "$(SITE_HANDLE)"
install: setup
	cd server && go run . setup -name "$(NAME)" -email "$(EMAIL)" -password "$(PASSWORD)" -site-name "$(SITE_NAME)" -site-handle "$(SITE_HANDLE)"

setup: server/.env site/.env server/node_modules site/node_modules
server/.env: ; cp server/.env.example $@
site/.env:   ; cp site/.env.example $@
server/node_modules: server/package.json ; cd server && npm i --silent
site/node_modules:   site/package.json   ; cd site && npm i --silent

PORT = $(shell sed -n 's/^ADDR=.*:\([0-9]*\)$$/\1/p' server/.env)
dev: setup
	@nc -z 127.0.0.1 3306 || { echo "MySQL 127.0.0.1:3306 belum jalan (DBngin)"; exit 1; }
	@! nc -z 127.0.0.1 $(PORT) || { echo "Port $(PORT) sudah dipakai:"; lsof -nP -iTCP:$(PORT) -sTCP:LISTEN | tail -n +2; echo "Ganti ADDR di server/.env dan CMS_URL di site/.env"; exit 1; }
	@trap '$(MAKE) -s stop' EXIT INT TERM; \
	( cd server && npx vite ) & \
	( cd site && npx astro dev ) & \
	( cd server && while true; do go run . & PID=$$!; \
	    fswatch -1 -r --exclude static --exclude node_modules . >/dev/null 2>&1 || { wait $$PID; break; }; \
	    kill $$PID 2>/dev/null; wait $$PID 2>/dev/null; echo "↻ restart go"; done ); \
	wait

# make stop → matikan semua yang dijalankan make dev (Go di $(PORT), Vite 5173, Astro 4321)
stop:
	@for p in $(PORT) 4321 5173; do lsof -nP -tiTCP:$$p -sTCP:LISTEN | xargs -r kill 2>/dev/null; done
	@pkill -f "node_modules/[.]bin/vite" 2>/dev/null; pkill -f "[a]stro dev" 2>/dev/null; pkill -f "[f]swatch -1 -r --exclude static" 2>/dev/null; pkill -f "[m]ake dev" 2>/dev/null; true
	@echo "stopped"

server: ; cd server && go run .
css:    ; cd server && npm run -s build
site:   ; cd site && npx astro dev
test:   ; cd server && go vet ./... && go test ./...

build: setup css
	cd server && go build -o ../dist/draftcms .
	cd site && npx astro build

clean: ; rm -rf dist server/static/app.css site/dist

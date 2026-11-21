# make dev   → MySQL check + Go server (auto-reload) + Vite (TS + Tailwind, HMR) + Astro dev, semua dalam satu terminal
# make build → binary Go + Vite build + Astro static build
SHELL := /bin/bash
.PHONY: dev stop build setup install fresh server css site test clean media media-remove sidebar activity_log

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
# cek port tanpa nc/lsof (tidak ada di semua mesin): pakai /dev/tcp bawaan bash
up = (exec 3<>/dev/tcp/127.0.0.1/$(1)) 2>/dev/null
dev: setup
	@$(call up,3306) || { echo "MySQL 127.0.0.1:3306 belum jalan (DBngin / mysql.server start)"; exit 1; }
	@! $(call up,$(PORT)) || { echo "Port $(PORT) sudah dipakai. Ganti ADDR di server/.env dan CMS_URL di site/.env"; command -v lsof >/dev/null && lsof -nP -iTCP:$(PORT) -sTCP:LISTEN | tail -n +2; exit 1; }
	@trap '$(MAKE) -s stop' EXIT INT TERM; \
	( cd server && npx vite ) & \
	( cd site && npx astro dev ) & \
	( cd server && while true; do go run . & PID=$$!; \
	    fswatch -1 -r --exclude static --exclude node_modules . >/dev/null 2>&1 || { wait $$PID; break; }; \
	    kill $$PID 2>/dev/null; wait $$PID 2>/dev/null; echo "↻ restart go"; done ); \
	wait

# make stop → matikan semua yang dijalankan make dev (Go di $(PORT), Vite 5173, Astro 4321)
stop:
	@command -v lsof >/dev/null && for p in $(PORT) 4321 5173; do lsof -nP -tiTCP:$$p -sTCP:LISTEN | xargs -r kill 2>/dev/null; done; true
	@pkill -f "[e]xe/draftcms-lite" 2>/dev/null; pkill -f "[g]o run \." 2>/dev/null; true
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

# make media "Blog Images" [TYPE=public|private]  → padanan `php artisan make:media` (judul/TYPE kosong = ditanya interaktif)
# make media-remove blog_images                   → padanan `php artisan remove:media`
# Kata setelah `media`/`media-remove` dibaca sebagai argumen, bukan target make. Di akhir file supaya default goal tetap `fresh`.
ifneq (,$(filter media media-remove,$(firstword $(MAKECMDGOALS))))
ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
.DEFAULT: ; @:
endif
TYPE ?=
media: setup ; cd server && go run . media -type "$(TYPE)" "$(ARGS)"
media-remove: setup ; cd server && go run . media:remove "$(ARGS)"

# make sidebar [FORCE=1]  → padanan `php artisan admin:sidebar [--force]` (FORCE=1 = reset tree sidebar ke default)
FORCE ?=
sidebar: setup ; cd server && go run . sidebar $(if $(FORCE),-force)

# make activity_log  → padanan `php artisan draft:activity_log`: set ACTIVITY_LOG_ENABLED=true di server/.env (restart server / make dev otomatis restart)
activity_log: setup ; cd server && go run . activity_log

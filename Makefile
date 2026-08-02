.PHONY: help setup doctor \
        backend_start backend_stop backend_status backend_restart backend_logs \
        frontend_start frontend_stop frontend_status frontend_restart frontend_logs \
        all_start all_stop all_restart all_status \
        force_stop_backend force_stop_frontend force_stop \
        health db_backup db_restore logs \
        clean clean_symlinks

# ============================
# Load .env
# ============================
-include .env
BINARY_NAME ?= fim-server
BACKEND_DIR = backend
FRONTEND_DIR = frontend
BACKEND_LOG = server.log
FRONTEND_LOG = frontend.log
FRONTEND_PORT ?= 3000
FRONTEND_HOST ?= localhost

# Backend port: .env sudah mendefinisikan BACKEND_PORT langsung (nama variabel
# sama persis), jadi -include .env di atas otomatis men-set nilai ini.
# Fallback 8080 hanya dipakai kalau .env tidak ada sama sekali.
BACKEND_PORT ?= 8080
BACKEND_HOST ?= 0.0.0.0

# PID files -> jauh lebih akurat daripada pgrep -f pattern matching
BACKEND_PID_FILE  = $(BACKEND_DIR)/.backend.pid
FRONTEND_PID_FILE = $(FRONTEND_DIR)/.frontend.pid

# Backup & health check config
BACKUP_DIR   ?= backups
# DB_FILE ikut BACKEND_DB_PATH dari .env kalau ada, supaya tidak pernah
# mismatch dengan path yang benar-benar dipakai binary Go.
ifdef BACKEND_DB_PATH
DB_FILE      ?= $(BACKEND_DB_PATH)
else
DB_FILE      ?= $(BACKEND_DIR)/data/ojs_monitor.db
endif
HEALTH_PATH  ?= /api/health
LOG_MAX_KB   ?= 10240
DEFAULT_SECRET_PLACEHOLDER := change-this-to-a-random-secret-key-in-production

# ============================
# Colors
# ============================
GREEN  := \033[0;32m
YELLOW := \033[0;33m
RED    := \033[0;31m
BLUE   := \033[0;34m
NC     := \033[0m

# ============================
# Help
# ============================
help:
	@echo ""
	@echo "$(GREEN)OJS Monitor - System Management$(NC)"
	@echo ""
	@echo "Binary: $(BINARY_NAME) | Backend port: $(BACKEND_PORT) | Frontend: $(FRONTEND_HOST):$(FRONTEND_PORT)"
	@echo ""
	@echo "$(BLUE)Setup:$(NC)"
	@echo "  $(YELLOW)make setup$(NC)            Install deps, prepare .env, create dirs"
	@echo "  $(YELLOW)make doctor$(NC)           Check environment (versions, .env, ports)"
	@echo ""
	@echo "$(BLUE)Backend:$(NC)"
	@echo "  $(YELLOW)make backend_start$(NC)    Start backend server"
	@echo "  $(YELLOW)make backend_stop$(NC)     Stop backend server"
	@echo "  $(YELLOW)make backend_status$(NC)   Check backend status"
	@echo "  $(YELLOW)make backend_restart$(NC)  Restart backend server"
	@echo "  $(YELLOW)make backend_logs$(NC)     View backend logs"
	@echo ""
	@echo "$(BLUE)Frontend:$(NC)"
	@echo "  $(YELLOW)make frontend_start$(NC)   Start frontend dev server"
	@echo "  $(YELLOW)make frontend_stop$(NC)    Stop frontend server"
	@echo "  $(YELLOW)make frontend_status$(NC)  Check frontend status"
	@echo "  $(YELLOW)make frontend_restart$(NC) Restart frontend server"
	@echo "  $(YELLOW)make frontend_logs$(NC)    View frontend logs"
	@echo ""
	@echo "$(BLUE)All:$(NC)"
	@echo "  $(YELLOW)make all_start$(NC)        Start backend + frontend"
	@echo "  $(YELLOW)make all_stop$(NC)         Stop backend + frontend"
	@echo "  $(YELLOW)make all_restart$(NC)      Restart backend + frontend"
	@echo "  $(YELLOW)make all_status$(NC)       Check all status"
	@echo "  $(YELLOW)make logs$(NC)             Tail backend + frontend logs together"
	@echo "  $(YELLOW)make health$(NC)           HTTP health check backend + frontend"
	@echo ""
	@echo "$(BLUE)Data:$(NC)"
	@echo "  $(YELLOW)make db_backup$(NC)        Backup SQLite DB to $(BACKUP_DIR)/"
	@echo "  $(YELLOW)make db_restore$(NC)       Restore DB (usage: make db_restore FILE=path)"
	@echo ""
	@echo "$(BLUE)Other:$(NC)"
	@echo "  $(YELLOW)make clean$(NC)            Clean build artifacts"
	@echo "  $(YELLOW)make force_stop$(NC)       Kill anything on backend/frontend ports (orphan cleanup)"
	@echo ""

# ============================
# Setup / Doctor
# ============================

setup:
	@echo "$(GREEN)Setting up project...$(NC)"
	@command -v go >/dev/null 2>&1 || { echo "$(RED)✗ Go tidak ditemukan. Install Go dulu.$(NC)"; exit 1; }
	@command -v node >/dev/null 2>&1 || { echo "$(RED)✗ Node.js tidak ditemukan. Install Node dulu.$(NC)"; exit 1; }
	@command -v npm >/dev/null 2>&1 || { echo "$(RED)✗ npm tidak ditemukan.$(NC)"; exit 1; }
	@if [ ! -f .env ]; then \
		if [ -f .env.example ]; then \
			cp .env.example .env; \
			echo "$(GREEN)✓ .env dibuat dari .env.example — cek dan sesuaikan isinya$(NC)"; \
		else \
			echo "$(YELLOW)⚠ .env tidak ada dan .env.example juga tidak ada. Buat .env manual.$(NC)"; \
		fi; \
	else \
		echo "$(GREEN)✓ .env sudah ada$(NC)"; \
	fi
	@mkdir -p $(BACKEND_DIR)/data $(BACKUP_DIR)
	@if [ -f $(BACKEND_DIR)/go.mod ]; then \
		echo "$(GREEN)Downloading Go modules...$(NC)"; \
		cd $(BACKEND_DIR) && go mod download; \
	fi
	@if [ -f $(FRONTEND_DIR)/package.json ]; then \
		echo "$(GREEN)Installing npm packages...$(NC)"; \
		cd $(FRONTEND_DIR) && npm install; \
	fi
	@echo "$(GREEN)✓ Setup selesai. Jalankan 'make doctor' untuk verifikasi.$(NC)"

doctor:
	@echo "$(BLUE)== Environment check ==$(NC)"
	@if command -v go >/dev/null 2>&1; then \
		echo "$(GREEN)✓ Go:$(NC) $$(go version)"; \
	else \
		echo "$(RED)✗ Go tidak terinstall$(NC)"; \
	fi
	@if command -v node >/dev/null 2>&1; then \
		echo "$(GREEN)✓ Node:$(NC) $$(node --version)"; \
	else \
		echo "$(RED)✗ Node.js tidak terinstall$(NC)"; \
	fi
	@if command -v npm >/dev/null 2>&1; then \
		echo "$(GREEN)✓ npm:$(NC) $$(npm --version)"; \
	else \
		echo "$(RED)✗ npm tidak terinstall$(NC)"; \
	fi
	@echo ""
	@echo "$(BLUE)== Config check ==$(NC)"
	@if [ -f .env ]; then \
		echo "$(GREEN)✓ .env ditemukan$(NC)"; \
	else \
		echo "$(RED)✗ .env tidak ditemukan — jalankan 'make setup'$(NC)"; \
	fi
	@if [ -f $(BACKEND_DIR)/go.mod ]; then \
		echo "$(GREEN)✓ backend/go.mod ditemukan$(NC)"; \
	else \
		echo "$(RED)✗ backend/go.mod tidak ditemukan$(NC)"; \
	fi
	@if [ -f $(FRONTEND_DIR)/package.json ]; then \
		echo "$(GREEN)✓ frontend/package.json ditemukan$(NC)"; \
	else \
		echo "$(RED)✗ frontend/package.json tidak ditemukan$(NC)"; \
	fi
	@if [ "$(BACKEND_SECRET_KEY)" = "$(DEFAULT_SECRET_PLACEHOLDER)" ]; then \
		echo "$(RED)✗ BACKEND_SECRET_KEY masih pakai nilai placeholder default!$(NC)"; \
		echo "$(YELLOW)  Ganti di .env sebelum ini jalan di luar mesin lokal Anda.$(NC)"; \
	elif [ -n "$(BACKEND_SECRET_KEY)" ]; then \
		echo "$(GREEN)✓ BACKEND_SECRET_KEY sudah di-custom$(NC)"; \
	fi
	@echo ""
	@echo "$(BLUE)== Port check ==$(NC)"
	@BOWNER=$$( (ss -ltnp 2>/dev/null || netstat -ltnp 2>/dev/null) | grep ":$(BACKEND_PORT) " | grep -oP '(?<=pid=)[0-9]+' | head -1 ); \
	if [ -n "$$BOWNER" ]; then \
		echo "$(YELLOW)⚠ Port $(BACKEND_PORT) (backend) sudah dipakai PID $$BOWNER$(NC)"; \
	else \
		echo "$(GREEN)✓ Port $(BACKEND_PORT) (backend) bebas$(NC)"; \
	fi
	@FOWNER=$$( (ss -ltnp 2>/dev/null || netstat -ltnp 2>/dev/null) | grep ":$(FRONTEND_PORT) " | grep -oP '(?<=pid=)[0-9]+' | head -1 ); \
	if [ -n "$$FOWNER" ]; then \
		echo "$(YELLOW)⚠ Port $(FRONTEND_PORT) (frontend) sudah dipakai PID $$FOWNER$(NC)"; \
	else \
		echo "$(GREEN)✓ Port $(FRONTEND_PORT) (frontend) bebas$(NC)"; \
	fi

# ============================
# Backend Commands
# ============================

backend_status:
	@if [ -f $(BACKEND_PID_FILE) ] && ps -p $$(cat $(BACKEND_PID_FILE)) > /dev/null 2>&1; then \
		PID=$$(cat $(BACKEND_PID_FILE)); \
		echo "$(GREEN)✓ Backend: RUNNING$(NC)"; \
		echo "  PID: $$PID"; \
		ps -p $$PID -o rss,%cpu,etime --no-headers 2>/dev/null | awk '{printf "  RSS: %.1f MB | CPU: %s%% | Uptime: %s\n", $$1/1024, $$2, $$3}'; \
	else \
		echo "$(RED)✗ Backend: NOT RUNNING$(NC)"; \
		rm -f $(BACKEND_PID_FILE); \
	fi

backend_start: clean_symlinks
	@if [ -f $(BACKEND_PID_FILE) ] && ps -p $$(cat $(BACKEND_PID_FILE)) > /dev/null 2>&1; then \
		echo "$(YELLOW)⚠ Backend already running (PID: $$(cat $(BACKEND_PID_FILE)))$(NC)"; \
		exit 0; \
	fi
	@OWNER=$$( (ss -ltnp 2>/dev/null || netstat -ltnp 2>/dev/null) | grep ":$(BACKEND_PORT) " | grep -oP '(?<=pid=)[0-9]+' | head -1 ); \
	if [ -n "$$OWNER" ]; then \
		echo "$(RED)✗ Port $(BACKEND_PORT) sudah dipakai proses lain (PID: $$OWNER), bukan oleh Makefile ini.$(NC)"; \
		ps -p $$OWNER -o pid,cmd --no-headers 2>/dev/null | sed 's/^/  /'; \
		echo "$(YELLOW)  Jalankan 'make force_stop_backend' untuk mematikan paksa, atau cek manual dengan: sudo lsof -i :$(BACKEND_PORT)$(NC)"; \
		exit 1; \
	fi
	@mkdir -p $(BACKEND_DIR)/data
	@if [ -f $(BACKEND_DIR)/$(BACKEND_LOG) ]; then \
		SIZE_KB=$$(du -k $(BACKEND_DIR)/$(BACKEND_LOG) 2>/dev/null | cut -f1); \
		if [ "$$SIZE_KB" -gt "$(LOG_MAX_KB)" ] 2>/dev/null; then \
			mv $(BACKEND_DIR)/$(BACKEND_LOG) $(BACKEND_DIR)/$(BACKEND_LOG).$$(date +%Y%m%d_%H%M%S).old; \
			echo "$(YELLOW)  Log backend dirotasi (>$(LOG_MAX_KB)KB)$(NC)"; \
		fi; \
	fi
	@echo "$(GREEN)Starting backend...$(NC)"
	@ln -sf ../.env $(BACKEND_DIR)/.env
	@cd $(BACKEND_DIR) && go build -o $(BINARY_NAME) .
	@cd $(BACKEND_DIR) && nohup ./$(BINARY_NAME) > $(BACKEND_LOG) 2>&1 & \
		echo $$! > $(BACKEND_PID_FILE)
	@sleep 2
	@if [ -f $(BACKEND_PID_FILE) ] && ps -p $$(cat $(BACKEND_PID_FILE)) > /dev/null 2>&1; then \
		echo "$(GREEN)✓ Backend started (PID: $$(cat $(BACKEND_PID_FILE)))$(NC)"; \
	else \
		echo "$(RED)✗ Backend failed to start$(NC)"; \
		cat $(BACKEND_DIR)/$(BACKEND_LOG) 2>/dev/null; \
		rm -f $(BACKEND_PID_FILE); \
	fi

backend_stop:
	@if [ -f $(BACKEND_PID_FILE) ] && ps -p $$(cat $(BACKEND_PID_FILE)) > /dev/null 2>&1; then \
		PID=$$(cat $(BACKEND_PID_FILE)); \
		echo "$(YELLOW)Stopping backend (PID: $$PID)...$(NC)"; \
		kill $$PID 2>/dev/null; \
		sleep 1; \
		if ps -p $$PID > /dev/null 2>&1; then \
			kill -9 $$PID 2>/dev/null; \
		fi; \
		rm -f $(BACKEND_PID_FILE); \
		echo "$(GREEN)✓ Backend stopped$(NC)"; \
	else \
		echo "$(YELLOW)Backend is not running$(NC)"; \
		rm -f $(BACKEND_PID_FILE); \
	fi

backend_restart: backend_stop
	@sleep 1
	@$(MAKE) backend_start

backend_logs:
	@if [ -f $(BACKEND_DIR)/$(BACKEND_LOG) ]; then \
		tail -f $(BACKEND_DIR)/$(BACKEND_LOG); \
	else \
		echo "$(RED)No log file found$(NC)"; \
	fi

# ============================
# Frontend Commands
# ============================

frontend_status:
	@if [ -f $(FRONTEND_PID_FILE) ] && ps -p $$(cat $(FRONTEND_PID_FILE)) > /dev/null 2>&1; then \
		PID=$$(cat $(FRONTEND_PID_FILE)); \
		echo "$(GREEN)✓ Frontend: RUNNING$(NC)"; \
		echo "  PID: $$PID"; \
		echo "  URL: http://$(FRONTEND_HOST):$(FRONTEND_PORT)"; \
		ps -p $$PID -o rss,%cpu --no-headers 2>/dev/null | awk '{printf "  RSS: %.1f MB | CPU: %s%%\n", $$1/1024, $$2}'; \
	else \
		echo "$(RED)✗ Frontend: NOT RUNNING$(NC)"; \
		echo "  Expected: http://$(FRONTEND_HOST):$(FRONTEND_PORT)"; \
		rm -f $(FRONTEND_PID_FILE); \
	fi

frontend_start: clean_symlinks
	@if [ -f $(FRONTEND_PID_FILE) ] && ps -p $$(cat $(FRONTEND_PID_FILE)) > /dev/null 2>&1; then \
		echo "$(YELLOW)⚠ Frontend already running (PID: $$(cat $(FRONTEND_PID_FILE)))$(NC)"; \
		exit 0; \
	fi
	@OWNER=$$( (ss -ltnp 2>/dev/null || netstat -ltnp 2>/dev/null) | grep ":$(FRONTEND_PORT) " | grep -oP '(?<=pid=)[0-9]+' | head -1 ); \
	if [ -n "$$OWNER" ]; then \
		echo "$(RED)✗ Port $(FRONTEND_PORT) sudah dipakai proses lain (PID: $$OWNER), bukan oleh Makefile ini.$(NC)"; \
		ps -p $$OWNER -o pid,cmd --no-headers 2>/dev/null | sed 's/^/  /'; \
		echo "$(YELLOW)  Jalankan 'make force_stop_frontend' untuk mematikan paksa, atau cek manual dengan: sudo lsof -i :$(FRONTEND_PORT)$(NC)"; \
		exit 1; \
	fi
	@mkdir -p $(FRONTEND_DIR)
	@if [ -f $(FRONTEND_DIR)/$(FRONTEND_LOG) ]; then \
		SIZE_KB=$$(du -k $(FRONTEND_DIR)/$(FRONTEND_LOG) 2>/dev/null | cut -f1); \
		if [ "$$SIZE_KB" -gt "$(LOG_MAX_KB)" ] 2>/dev/null; then \
			mv $(FRONTEND_DIR)/$(FRONTEND_LOG) $(FRONTEND_DIR)/$(FRONTEND_LOG).$$(date +%Y%m%d_%H%M%S).old; \
			echo "$(YELLOW)  Log frontend dirotasi (>$(LOG_MAX_KB)KB)$(NC)"; \
		fi; \
	fi
	@echo "$(GREEN)Starting frontend...$(NC)"
	@ln -sf ../../.env $(FRONTEND_DIR)/.env.local
	@cd $(FRONTEND_DIR) && PORT=$(FRONTEND_PORT) HOST=$(FRONTEND_HOST) setsid npm run dev > $(FRONTEND_LOG) 2>&1 < /dev/null & \
		echo $$! > $(FRONTEND_PID_FILE)
	@sleep 2
	@if [ -f $(FRONTEND_PID_FILE) ] && ps -p $$(cat $(FRONTEND_PID_FILE)) > /dev/null 2>&1; then \
		echo "$(GREEN)✓ Frontend started (PID: $$(cat $(FRONTEND_PID_FILE)))$(NC)"; \
		echo "  URL: http://$(FRONTEND_HOST):$(FRONTEND_PORT)"; \
	else \
		echo "$(RED)✗ Frontend failed to start$(NC)"; \
		cat $(FRONTEND_DIR)/$(FRONTEND_LOG) 2>/dev/null; \
		rm -f $(FRONTEND_PID_FILE); \
	fi

frontend_stop:
	@if [ -f $(FRONTEND_PID_FILE) ] && ps -p $$(cat $(FRONTEND_PID_FILE)) > /dev/null 2>&1; then \
		PID=$$(cat $(FRONTEND_PID_FILE)); \
		echo "$(YELLOW)Stopping frontend (PID: $$PID)...$(NC)"; \
		kill -TERM -$$PID 2>/dev/null || kill $$PID 2>/dev/null; \
		sleep 1; \
		if ps -p $$PID > /dev/null 2>&1; then \
			kill -9 -$$PID 2>/dev/null || kill -9 $$PID 2>/dev/null; \
		fi; \
		rm -f $(FRONTEND_PID_FILE); \
		echo "$(GREEN)✓ Frontend stopped$(NC)"; \
	else \
		echo "$(YELLOW)Frontend is not running (menurut PID file)$(NC)"; \
		rm -f $(FRONTEND_PID_FILE); \
	fi
	@PORT_OWNER=$$( (ss -ltnp 2>/dev/null || netstat -ltnp 2>/dev/null) | grep ":$(FRONTEND_PORT) " | grep -oP '(?<=pid=)[0-9]+' | head -1 ); \
	if [ -n "$$PORT_OWNER" ]; then \
		echo "$(YELLOW)  Masih ada child/orphan process nempel di port $(FRONTEND_PORT) (PID $$PORT_OWNER), dibersihkan juga...$(NC)"; \
		kill -9 $$PORT_OWNER 2>/dev/null || sudo kill -9 $$PORT_OWNER 2>/dev/null; \
	fi

frontend_restart: frontend_stop
	@sleep 1
	@$(MAKE) frontend_start

frontend_logs:
	@if [ -f $(FRONTEND_DIR)/$(FRONTEND_LOG) ]; then \
		tail -f $(FRONTEND_DIR)/$(FRONTEND_LOG); \
	else \
		echo "$(RED)No log file found$(NC)"; \
	fi

# ============================
# Force stop by port (buat bersihin proses orphan yang tidak
# tercatat di PID file, mis. sisa run sebelum pakai versi Makefile ini)
# ============================

force_stop_backend:
	@OWNER=$$( (ss -ltnp 2>/dev/null || netstat -ltnp 2>/dev/null) | grep ":$(BACKEND_PORT) " | grep -oP '(?<=pid=)[0-9]+' | head -1 ); \
	if [ -n "$$OWNER" ]; then \
		echo "$(YELLOW)Membunuh proses di port $(BACKEND_PORT) (PID: $$OWNER)...$(NC)"; \
		kill -9 $$OWNER 2>/dev/null || sudo kill -9 $$OWNER 2>/dev/null; \
		echo "$(GREEN)✓ Selesai$(NC)"; \
	else \
		echo "$(YELLOW)Tidak ada proses di port $(BACKEND_PORT)$(NC)"; \
	fi
	@rm -f $(BACKEND_PID_FILE)

force_stop_frontend:
	@OWNER=$$( (ss -ltnp 2>/dev/null || netstat -ltnp 2>/dev/null) | grep ":$(FRONTEND_PORT) " | grep -oP '(?<=pid=)[0-9]+' | head -1 ); \
	if [ -n "$$OWNER" ]; then \
		echo "$(YELLOW)Membunuh proses di port $(FRONTEND_PORT) (PID: $$OWNER)...$(NC)"; \
		kill -9 $$OWNER 2>/dev/null || sudo kill -9 $$OWNER 2>/dev/null; \
		echo "$(GREEN)✓ Selesai$(NC)"; \
	else \
		echo "$(YELLOW)Tidak ada proses di port $(FRONTEND_PORT)$(NC)"; \
	fi
	@rm -f $(FRONTEND_PID_FILE)

force_stop: force_stop_backend force_stop_frontend

# ============================
# All Commands
# ============================

all_start: backend_start
	@$(MAKE) frontend_start

all_stop: backend_stop
	@$(MAKE) frontend_stop

all_restart: backend_restart
	@$(MAKE) frontend_restart

all_status:
	@$(MAKE) backend_status
	@$(MAKE) frontend_status

logs:
	@BLOG=$(BACKEND_DIR)/$(BACKEND_LOG); \
	FLOG=$(FRONTEND_DIR)/$(FRONTEND_LOG); \
	FILES=""; \
	[ -f $$BLOG ] && FILES="$$FILES $$BLOG"; \
	[ -f $$FLOG ] && FILES="$$FILES $$FLOG"; \
	if [ -z "$$FILES" ]; then \
		echo "$(RED)Belum ada log file. Jalankan server dulu.$(NC)"; \
	else \
		tail -f $$FILES; \
	fi

# ============================
# Health check
# ============================

health:
	@echo "$(BLUE)== Backend health ($(HEALTH_PATH)) ==$(NC)"
	@if command -v curl >/dev/null 2>&1; then \
		CODE=$$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 http://localhost:$(BACKEND_PORT)$(HEALTH_PATH) 2>/dev/null); \
		if [ "$$CODE" = "200" ]; then \
			echo "$(GREEN)✓ Backend OK (HTTP $$CODE)$(NC)"; \
		elif [ -z "$$CODE" ] || [ "$$CODE" = "000" ]; then \
			echo "$(RED)✗ Backend tidak merespon di port $(BACKEND_PORT)$(NC)"; \
		else \
			echo "$(YELLOW)⚠ Backend merespon tapi HTTP $$CODE (cek path $(HEALTH_PATH), sesuaikan HEALTH_PATH kalau beda)$(NC)"; \
		fi; \
	else \
		echo "$(YELLOW)curl tidak terinstall, skip health check$(NC)"; \
	fi
	@echo "$(BLUE)== Frontend health ==$(NC)"
	@if command -v curl >/dev/null 2>&1; then \
		CODE=$$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 http://$(FRONTEND_HOST):$(FRONTEND_PORT) 2>/dev/null); \
		if [ "$$CODE" = "200" ] || [ "$$CODE" = "307" ] || [ "$$CODE" = "308" ]; then \
			echo "$(GREEN)✓ Frontend OK (HTTP $$CODE)$(NC)"; \
		elif [ -z "$$CODE" ] || [ "$$CODE" = "000" ]; then \
			echo "$(RED)✗ Frontend tidak merespon di port $(FRONTEND_PORT)$(NC)"; \
		else \
			echo "$(YELLOW)⚠ Frontend merespon tapi HTTP $$CODE$(NC)"; \
		fi; \
	fi

# ============================
# Database backup/restore
# ============================

db_backup:
	@if [ ! -f $(DB_FILE) ]; then \
		echo "$(RED)✗ DB tidak ditemukan di $(DB_FILE)$(NC)"; \
		exit 1; \
	fi
	@mkdir -p $(BACKUP_DIR)
	@STAMP=$$(date +%Y%m%d_%H%M%S); \
	DEST=$(BACKUP_DIR)/ojs_monitor_$$STAMP.db; \
	cp $(DB_FILE) $$DEST; \
	[ -f $(DB_FILE)-wal ] && cp $(DB_FILE)-wal $$DEST-wal 2>/dev/null; \
	[ -f $(DB_FILE)-shm ] && cp $(DB_FILE)-shm $$DEST-shm 2>/dev/null; \
	echo "$(GREEN)✓ Backup dibuat: $$DEST$(NC)"; \
	echo "$(YELLOW)  Catatan: backend masih jalan saat backup ini dibuat. Untuk hasil paling konsisten, 'make backend_stop' dulu sebelum backup.$(NC)"

db_restore:
	@if [ -z "$(FILE)" ]; then \
		echo "$(RED)✗ Usage: make db_restore FILE=$(BACKUP_DIR)/nama_backup.db$(NC)"; \
		ls -1 $(BACKUP_DIR)/*.db 2>/dev/null | sed 's/^/  Tersedia: /'; \
		exit 1; \
	fi
	@if [ ! -f "$(FILE)" ]; then \
		echo "$(RED)✗ File $(FILE) tidak ditemukan$(NC)"; \
		exit 1; \
	fi
	@if [ -f $(BACKEND_PID_FILE) ] && ps -p $$(cat $(BACKEND_PID_FILE)) > /dev/null 2>&1; then \
		echo "$(RED)✗ Backend masih jalan. Jalankan 'make backend_stop' dulu sebelum restore.$(NC)"; \
		exit 1; \
	fi
	@cp "$(FILE)" $(DB_FILE)
	@echo "$(GREEN)✓ DB direstore dari $(FILE)$(NC)"

# ============================
# Clean
# ============================

clean_symlinks:
	@rm -f $(BACKEND_DIR)/.env $(FRONTEND_DIR)/.env.local 2>/dev/null; true

clean:
	@echo "$(YELLOW)Cleaning...$(NC)"
	@rm -f $(BACKEND_DIR)/$(BINARY_NAME) $(BACKEND_DIR)/server 2>/dev/null
	@rm -f $(BACKEND_DIR)/$(BACKEND_LOG) $(FRONTEND_DIR)/$(FRONTEND_LOG) 2>/dev/null
	@rm -f $(BACKEND_DIR)/$(BACKEND_LOG).*.old $(FRONTEND_DIR)/$(FRONTEND_LOG).*.old 2>/dev/null
	@rm -f $(BACKEND_DIR)/.env $(FRONTEND_DIR)/.env.local 2>/dev/null
	@rm -f $(BACKEND_PID_FILE) $(FRONTEND_PID_FILE) 2>/dev/null
	@echo "$(GREEN)✓ Cleaned$(NC)"
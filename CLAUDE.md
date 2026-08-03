# CLAUDE.md

## Project

**OJS Monitor** - File Integrity Monitoring (FIM) untuk Open Journal Systems

Repository root:

```
/home/arissupriy/stai/ojs-monitor
```

**Fokus saat ini: Backend only (Phase by Phase)**

Frontend tidak dikerjakan sampai backend phase selesai.

---

# Workspace Boundary (STRICT)

Workspace yang diizinkan hanya:

```
/home/arissupriy/stai/ojs-monitor
```

Dilarang:

- membaca project lain
- membuat file di luar repository
- build di luar repository
- menggunakan /tmp
- menggunakan ~/Desktop
- menggunakan ~/Downloads
- mengakses parent directory
- menyentuh frontend/

Selalu gunakan path relatif.

---

# Working Directory

Sebelum menjalankan command, pastikan working directory adalah:

```
/home/arissupriy/stai/ojs-monitor
```

---

# Project Layout (Backend Only)

```
backend/
├── main.go           # Entry point, router, middleware
├── auth.go           # JWT authentication
├── db.go             # Database, migrations
├── handlers.go       # HTTP handlers (API)
├── models.go         # Data structures
├── scanner.go        # File scanning, OJS reconciliation
├── watcher.go        # Real-time FIM (inotifywait)
├── worker.go         # Background job queue
├── database/         # SQLite database
└── data/             # Data files

# Akan ditambahkan per fase:
# alerts/            # Phase 2: Alert system
# audit/             # Phase 2: auditd integration
# reports/           # Phase 3: Compliance reports
```

---

# Current Phase

Baca `NEXT_PLANS.md` untuk phase saat ini yang sedang dikerjakan.

Kerjakan **satu phase sampai selesai** sebelum pindah ke phase berikutnya.

---

# Primary Goal

Project ini merupakan File Integrity Monitoring (FIM).

Prioritas utama (berurutan):

1. **correctness** - FIM must be accurate
2. **security** - No false negatives
3. **reliability** - Deterministic behavior
4. **low false positive** - Avoid alert fatigue

Optimisasi performa hanya dilakukan jika tidak mengurangi correctness.

---

# Coding Rules

Ikuti style Go yang sudah ada.

Gunakan:

- gofmt
- idiomatic Go
- error handling eksplisit
- context jika diperlukan
- early return

Hindari:

- panic
- global mutable state
- magic number
- duplicate logic

---

# Build Rules

Seluruh build dilakukan dari `backend/`:

```bash
cd backend
go build ./...
go test ./...
go test ./... -race   # Jika menyentuh concurrency
```

Binary output:

```bash
go build -o ./backend/fim-server
```

---

# File Integrity Rules

Jangan mengubah tanpa memahami dampaknya:

- hashing algorithm
- event ordering
- audit correlation logic
- watcher logic

Setiap perubahan harus menjaga:

- deterministic output
- repeatable scan
- stable hash calculation

---

# Database Rules

Database SQLite:

```
backend/database/ojs_monitor.db
```

Jangan:

- hapus database
- overwrite database
- reset WAL
- migrate schema secara sembarangan

kecuali diminta.

---

# Security Rules

Selalu perhatikan:

- path traversal
- symlink attack
- TOCTOU
- race condition
- permission issue
- command injection
- resource exhaustion

Selalu anggap input tidak terpercaya.

---

# Modification Policy

Lakukan perubahan **sekecil mungkin**.

Jangan:

- refactor besar
- rename massal
- ubah struktur project
- ubah API publik

kecuali diminta.

---

# Git Policy

Diizinkan:

```
git diff
git status
git show
git log
```

Dilarang (kecuali diminta):

```
git push
git merge
git rebase
git tag
git force push
```

---

# Output Format

Saat selesai selalu laporkan:

1. Ringkasan perubahan
2. File yang diubah
3. Alasan perubahan
4. Validasi yang dijalankan
5. Risiko yang tersisa

Jangan mengatakan sesuatu berhasil apabila belum diverifikasi.

---

# Engineering Mindset

Sebelum mengubah kode:

1. Pahami arsitektur
2. Cari akar masalah
3. Buat perubahan minimal
4. Verifikasi hasil
5. Hindari regresi

Selalu utamakan correctness dibanding kecepatan implementasi.

---

# Build & Test Policy

Gunakan **Makefile** sebagai entry point utama untuk operasi project.

Jangan menjalankan command manual apabila target Makefile sudah tersedia.

Prioritaskan:

```bash
make doctor
make backend_start
make backend_stop
make backend_restart
make backend_status
make backend_logs
make health
make clean
```

---

# Backend Build

Backend selalu dibangun menjadi binary:

```
backend/fim-server
```

Gunakan:

```bash
cd backend
go build -o fim-server .
```

atau target Makefile yang sesuai apabila tersedia.

Jangan:

- mengubah nama binary
- membuat binary lain
- membuat binary di luar `backend/`

---

# Testing Before Build

Setiap perubahan backend wajib menjalankan:

```bash
cd backend
go test ./...
```

Jika perubahan menyentuh:

- goroutine
- worker
- watcher
- channel
- concurrency

maka wajib menjalankan:

```bash
cd backend
go test -race ./...
```

Binary hanya boleh dibangun apabila seluruh test berhasil.

---

# Validation

Sebelum task dianggap selesai, lakukan validasi berikut:

1. Build berhasil.
2. Seluruh test lulus.
3. Tidak ada race condition (jika relevan).
4. Binary berhasil dibuat:

```
backend/fim-server
```

5. Jalankan:

```bash
make health
```

jika backend dijalankan.

---

# Makefile Policy

Apabila workflow baru sering digunakan, **update Makefile** daripada menuliskan command panjang di dokumentasi atau README.

Utamakan otomatisasi melalui target Makefile daripada command manual yang berulang.


backend_test:
	cd backend && go test ./...

backend_race:
	cd backend && go test -race ./...

backend_build:
	cd backend && go build -o fim-server .

backend_verify: backend_test backend_build
	@echo "✓ Backend verified"

backend_verify_race: backend_race backend_build
	@echo "✓ Backend verified (race)"
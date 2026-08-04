# FIM Monitor

**A Generic File Integrity Monitoring Platform** with CMS-specific templates.

> OJS Monitor is the first dedicated template. The platform is designed to support multiple CMS/systems (WordPress, Drupal, custom) via template plugins.

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)
![Next.js](https://img.shields.io/badge/Next.js-16-black.svg)

---

## Platform Concept

```
┌─────────────────────────────────────────────────────────────────┐
│                     FIM MONITOR PLATFORM                       │
│              (Generic File Integrity Monitoring)                  │
├─────────────────────────────────────────────────────────────┤
│  Core Engine (Platform-wide)                                   │
│  ├── File Scanner (Hash, Permission, Metadata)                 │
│  ├── FIM Watcher (inotifywait)                                │
│  ├── Background Worker (Job Queue)                             │
│  └── Database (SQLite - Platform Schema)                       │
├─────────────────────────────────────────────────────────────┤
│  Templates/Plugins (CMS-Specific Detection)                   │
│  ├── OJS Template       → submission_files, users, journals   │
│  ├── WordPress Template → wp_posts, wp_uploads (future)       │
│  └── Custom Template   → (user-defined rules)                 │
└─────────────────────────────────────────────────────────────┘
```

---

## Features

### File Integrity Monitoring (FIM)
- **Baseline Scanning**: Create initial file baseline with SHA-256 hashes
- **Change Detection**: Detect added, modified, and deleted files
- **Permission Tracking**: Monitor file mode, UID, GID changes
- **Real-time Monitoring**: Using inotifywait for instant detection
- **Whitelist/Blacklist**: Exclude specific paths or extensions

### CMS-Specific Templates
- **OJS Template**: Orphan detection, user metrics, journal statistics
- **Extensible**: Add new CMS support via template interface
- **Generic Core**: Scanner/watcher work without CMS templates

### Async Job Processing
- **Background Worker**: Non-blocking scan jobs
- **Progress Tracking**: Real-time scan progress updates
- **Scheduled Scans**: Daily integrity scans
- **Graceful Shutdown**: Clean worker shutdown on signals

### Security
- **JWT Authentication**: Secure admin login
- **Audit Logging**: Track all admin actions
- **Race Condition Protection**: Database-level locking

---

## Architecture

### Clean Architecture

```
backend/
├── cmd/                    # Entry points
│   ├── manage/            # CLI tool
│   ├── server/             # HTTP API server
│   └── worker/             # Background worker
│
├── pkg/                    # Shared packages
│   └── response/          # HTTP response helpers
│
└── internal/              # Application code
    ├── domain/            # Business rules (models, interfaces)
    ├── application/        # Use cases
    ├── infrastructure/     # Technology implementations
    │   ├── database/      # SQLite/MySQL
    │   ├── http/           # HTTP handlers
    │   ├── scanner/        # File scanning
    │   ├── watcher/        # FIM watcher
    │   ├── worker/         # Background worker
    │   └── templates/      # CMS templates
    └── wire/               # Dependency injection
```

### System Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Frontend      │────▶│   Go Backend    │────▶│   SQLite DB     │
│   (Next.js)     │     │   (chi router)  │     │   (metadata)    │
└─────────────────┘     └────────┬────────┘     └─────────────────┘
                                 │
                    ┌────────────┼────────────┐
                    │ OJS Metrics│ CMS-specific│
                    ▼            ▼            ▼
             ┌──────────┐ ┌──────────┐ ┌──────────┐
             │   OJS    │ │WordPress │ │ Custom   │
             │ Template │ │ Template │ │ Template │
             └────┬─────┘ └────┬─────┘ └────┬─────┘
                  │             │             │
                  └─────────────┴─────────────┘
                              │
                              ▼
                      ┌─────────────┐
                      │ CMS MySQL   │
                      │ Databases   │
                      └─────────────┘
```

---

## Tech Stack

| Component | Technology |
|-----------|------------|
| Frontend | Next.js 16 (Turbopack), React, Tailwind CSS |
| Backend | Go 1.21+, chi router, bcrypt, jwt |
| Local DB | SQLite (WAL mode) |
| Target DB | MySQL 5.7+ (CMS databases) |

---

## Prerequisites

- **Go** 1.21 or higher
- **Node.js** 18 or higher
- **MySQL** 5.7+ (for CMS database access)
- **Linux** with inotifywait (for real-time monitoring)

---

## Quick Start

### 1. Clone and Build

```bash
git clone https://github.com/your-repo/fim-monitor.git
cd fim-monitor
make build
```

### 2. Setup Database

```bash
# Run migrations
./manage migrate

# Create admin user
./manage add-admin admin secretpassword

# Or use default admin
./manage seed  # admin / admin123
```

### 3. Start Services

```bash
# Terminal 1: API Server
./fim-server

# Terminal 2: Background Worker
./worker
```

### 4. Access

- **API Server**: http://localhost:8080
- **Frontend**: http://localhost:3000 (if running)
- **Default Login**: admin / admin123

---

## CLI Commands

```bash
# Database management
./manage migrate                     # Run migrations
./manage seed                      # Seed default admin
./manage add-admin <username> <password>  # Create admin user
./manage status                    # Show system status

# Server management (daemon mode)
./manage server:start             # Start server + worker
./manage server:stop              # Stop server + worker
./manage server:restart           # Restart all services
./manage server:status           # Show service status

# Manual mode (separate terminals)
./fim-server                      # Start HTTP API server
./worker                         # Start background worker

# Help
./manage help                    # Show all commands
```

---

## Server Management

The CLI manages server and worker processes using **PID files**:

| Command | Description |
|---------|-------------|
| `manage server:start` | Start server + worker (daemon) |
| `manage server:stop` | Stop gracefully with SIGTERM |
| `manage server:restart` | Restart all services |
| `manage server:status` | Show running processes |

PID files: `.server.pid`, `.worker.pid`

---

## Project Setup

### Create a New Project

1. Login to the dashboard
2. Click **"Add Project"**
3. Configure:
   - **Name**: Project identifier
   - **Template**: OJS (or future templates)
   - **App Path**: Path to CMS installation
   - **Files Path**: Path to CMS uploads
   - **Database**: CMS MySQL connection

### Start Baseline Scan

1. Open the project
2. Click **"Start Scan"**
3. Monitor progress in Jobs tab

---

## Template System

### How Templates Work

Templates provide CMS-specific detection:

- **Orphan Detection**: Files not in CMS database
- **User Metrics**: New users, validated users, etc.
- **CMS Statistics**: Journal counts, submission counts

### Available Templates

| Template | Status | Features |
|----------|--------|----------|
| OJS | ✅ Ready | Full support |
| WordPress | 🔜 Future | Pending |
| Drupal | 🔜 Future | Pending |
| Custom | 🔜 Future | User-defined |

### Adding a Template

1. Create `internal/templates/<name>/`
2. Implement `Template` interface
3. Register in application

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Backend server port |
| `DB_PATH` | `./database/` | SQLite database path |

### Permissions

For monitoring CMS paths, ensure:
- Read access to application directories
- Read access to upload directories
- MySQL SELECT permissions on CMS database

---

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/login` | Admin login |
| GET | `/api/projects` | List projects |
| POST | `/api/projects` | Create project |
| GET | `/api/projects/:id` | Get project |
| PUT | `/api/projects/:id` | Update project |
| POST | `/api/projects/:id/scan` | Start baseline scan |
| GET | `/api/projects/:id/jobs` | Get scan jobs |
| GET | `/api/projects/:id/files` | Get files |
| GET | `/api/logs` | Get audit logs |

---

## Development

### Makefile Targets

```bash
make build         # Build all binaries
make clean         # Remove binaries
make test          # Run tests
make test-race     # Run tests with race detector
make status        # Check system status
make migrate       # Run migrations
```

### Project Structure

```
backend/
├── cmd/                    # Binaries
│   ├── manage/main.go     # CLI tool
│   ├── server/main.go     # HTTP server
│   └── worker/main.go     # Background worker
├── pkg/                    # Shared
├── internal/               # Application
│   ├── domain/            # Business rules
│   ├── application/        # Use cases
│   └── infrastructure/    # Implementations
└── wire/                  # DI
```

---

## Security

1. **Change default password** after first login
2. **Use HTTPS** in production
3. **Restrict paths** - Read-only access to CMS directories
4. **MySQL permissions** - Use minimal required permissions

### Recommended MySQL Permissions

```sql
GRANT SELECT ON cms_database.* TO 'fim_monitor'@'%';
FLUSH PRIVILEGES;
```

---

## Troubleshooting

### "Connection timeout" when testing CMS database
- Verify MySQL is running and accessible
- Check firewall settings
- Ensure MySQL allows remote connections

### Scan stuck at "counting" status
- Check if worker is running
- Verify paths are correct and accessible
- Check disk space

### Files not detected
- Ensure paths have correct permissions
- Check whitelist/blacklist settings
- Verify symlinks are not being skipped

---

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

---

## License

MIT License - see [LICENSE](LICENSE) for details.

---

## Build Output

Binaries are located in `backend/`:

```
backend/manage       # CLI tool
backend/fim-server  # HTTP API server
backend/worker      # Background worker
```

Build using Makefile:

```bash
make build
```

Or manually:

```bash
cd backend
go build -o manage ./cmd/manage
go build -o fim-server ./cmd/server
go build -o worker ./cmd/worker
```

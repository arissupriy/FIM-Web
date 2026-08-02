# OJS Monitor

A self-hosted file integrity monitoring (FIM) and database audit system for Open Journal Systems (OJS).

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)
![Next.js](https://img.shields.io/badge/Next.js-16-black.svg)

## Features

### File Integrity Monitoring (FIM)
- **Baseline Scanning**: Create initial file baseline with SHA-256 hashes
- **Change Detection**: Detect added, modified, and deleted files
- **Whitelist Support**: Exclude specific paths from monitoring
- **Extension Blacklist**: Ignore files by extension (e.g., `.php`, `.phtml`)
- **Orphan Detection**: Identify uploaded files not registered in OJS database

### Database Auditing (OJS)
- **User Activity**: Track new users, validated users, disabled accounts
- **Admin Monitoring**: Count active administrators
- **Self-Registration Audit**: Detect insecure self-registration settings
- **Upload Tracking**: Monitor uploads by newly registered users

### Async Job Processing
- **Background Worker**: Non-blocking scan jobs
- **Progress Tracking**: Real-time scan progress updates
- **Automatic Rescan**: Scheduled rescans every 10 minutes
- **Graceful Shutdown**: Clean worker shutdown on SIGINT/SIGTERM

### Security
- **JWT Authentication**: Secure admin login
- **Audit Logging**: Track all admin actions
- **Race Condition Protection**: Database-level locking for concurrent jobs

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Frontend      │────▶│   Go Backend    │────▶│   SQLite DB     │
│   (Next.js)     │     │   (chi router)  │     │   (metadata)    │
└─────────────────┘     └────────┬────────┘     └─────────────────┘
                                 │
                                 │ OJS Metrics
                                 ▼
                        ┌─────────────────┐
                        │   MySQL/OJS     │
                        │   Database      │
                        └─────────────────┘
```

## Tech Stack

| Component | Technology |
|-----------|------------|
| Frontend | Next.js 16 (Turbopack), React, Tailwind CSS, Lucide Icons |
| Backend | Go 1.21+, chi router, bcrypt, jwt |
| Local DB | SQLite (WAL mode) |
| Target DB | MySQL 5.7+ (OJS database) |

## Prerequisites

- **Go** 1.21 or higher
- **Node.js** 18 or higher
- **MySQL** 5.7+ (for OJS database access)
- **Git**

## Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/your-repo/ojs-monitor.git
cd ojs-monitor
```

### 2. Configure Environment

```bash
# Edit .env in root directory
cp .env .env  # .env already exists, edit as needed
nano .env
```

### 3. Start Backend & Frontend

```bash
# Start backend server (auto-builds and runs)
make start

# Check status
make status

# View logs
make logs

# Start frontend (in another terminal)
make dev

# Stop backend
make stop
```

The backend runs on `http://localhost:8080`.
The frontend runs on `http://localhost:3000`.

### 4. Login

Default credentials:
- **Username**: `admin`
- **Password**: `admin123`

> ⚠️ Change the default password immediately in production!

## Project Setup

### Create a New Project

1. Click **"Add Project"** on the dashboard
2. Enter project name and description
3. Configure:
   - **App Path**: Path to OJS installation (e.g., `/var/www/ojs`)
   - **Files Path**: Path to OJS uploads (e.g., `/var/www/ojs/files`)
   - **Database Host**: OJS MySQL host
   - **Database Name**: OJS database name
   - **Database User**: MySQL username
   - **Database Password**: MySQL password

### Start Initial Scan

1. Open the project
2. Click **"Start Scan"**
3. The scan runs in background - progress shown in Jobs tab

## Deployment

### Production Build

#### Backend

```bash
cd backend

# Build for Linux AMD64
make build-linux

# Copy to server
scp server-linux user@server:/opt/ojs-monitor/server
```

#### Frontend

```bash
cd frontend

# Build for production
npm run build

# The output is in .next/
# Deploy to Nginx/Caddy/Vercel
```

### systemd Service (Backend)

Create `/etc/systemd/system/ojs-monitor.service`:

```ini
[Unit]
Description=OJS Monitor Backend
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/ojs-monitor
ExecStart=/opt/ojs-monitor/server
Restart=always
RestartSec=5
EnvironmentFile=/opt/ojs-monitor/.env

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable ojs-monitor
sudo systemctl start ojs-monitor
```

### Nginx Configuration (Frontend)

```nginx
server {
    listen 80;
    server_name ojs-monitor.example.com;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }

    location /api {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
    }
}
```

### Docker (Optional)

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  backend:
    build: ./backend
    ports:
      - "8080:8080"
    volumes:
      - ./backend/data:/app/data
      - /var/www/ojs:/var/www/ojs:ro
    depends_on:
      - mysql
    env_file:
      - ./backend/.env

  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
    depends_on:
      - backend

  mysql:
    image: mysql:5.7
    environment:
      MYSQL_ROOT_PASSWORD: root_password
      MYSQL_DATABASE: ojs
      MYSQL_USER: ojs_user
      MYSQL_PASSWORD: your_password
    volumes:
      - mysql_data:/var/lib/mysql

volumes:
  mysql_data:
```

## Configuration

### Environment Variables

Create a `.env` file (copy from `.env.example`) to configure the backend:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Backend server port |
| `HOST` | `0.0.0.0` | Server bind address |
| `SECRET_KEY` | (required) | JWT signing secret |
| `DB_PATH` | `./data/ojs_monitor.db` | SQLite database path |
| `LOCALE` | `id-ID` | Date/time formatting locale |
| `FIM_BUFFER_SIZE` | `1000` | FIM event buffer size |
| `FIM_BATCH_INTERVAL_MS` | `1000` | Batch processing interval |
| `FIM_DEBOUNCE_MS` | `500` | Event debounce window |
| `LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `CORS_ORIGINS` | `http://localhost:3000` | Allowed CORS origins |
| `SESSION_TIMEOUT_MINUTES` | `60` | Session timeout |
| `MAX_FILE_SIZE_MB` | `100` | Max file size to hash |
| `OJS_DB_TIMEOUT_SECONDS` | `10` | OJS database connection timeout |

### Database Directory

SQLite database is stored in `./data/ojs_monitor.db`. This directory is gitignored.

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/login` | Admin login |
| GET | `/api/projects` | List all projects |
| POST | `/api/projects` | Create new project |
| GET | `/api/projects/:id` | Get project details |
| PUT | `/api/projects/:id` | Update project |
| POST | `/api/projects/:id/scan` | Start baseline scan |
| GET | `/api/projects/:id/jobs` | Get scan jobs |
| GET | `/api/projects/:id/files` | Get monitored files |
| GET | `/api/projects/:id/audit` | Get audit metrics |
| GET | `/api/logs` | Get audit logs |
| POST | `/api/test-connection` | Test OJS connection |

## Security Considerations

1. **Change default admin password** immediately after first login
2. **Use HTTPS** in production (configure reverse proxy)
3. **Restrict file paths** - App should have read-only access to OJS paths
4. **Database permissions** - Use MySQL user with minimal required permissions

### Recommended MySQL User Permissions

```sql
GRANT SELECT ON ojs_database.* TO 'ojs_monitor'@'%';
FLUSH PRIVILEGES;
```

## Troubleshooting

### "Connection timeout" when testing OJS database
- Verify MySQL is running and accessible
- Check firewall settings
- Ensure MySQL allows remote connections (or use socket)

### Scan stuck at "counting" status
- Check if worker is running (backend logs)
- Verify paths are correct and accessible
- Check disk space

### Files not detected
- Ensure paths have correct permissions
- Check whitelist/blacklist settings
- Verify symlinks are not being skipped

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## Support

For issues and feature requests, please open an issue on GitHub.

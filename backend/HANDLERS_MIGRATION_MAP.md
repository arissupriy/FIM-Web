# Handlers Migration Map

## Overview

File: `backend/handlers.go` (1324 lines, 25 functions)
Target: Clean Architecture handlers di `internal/infrastructure/http/handlers/`

## Mapping: Legacy → Clean Architecture

### ✅ Selesai (Clean Architecture Handler Ada)

| Legacy Handler | Clean Architecture | Status |
|----------------|-------------------|--------|
| `handleLogin` | `AuthHandler.Login` | ✓ |
| `handleGetLogs` | `AuthHandler.GetAuditLogs` | ✓ |
| `handleGetProjects` | `ProjectHandler.List` | ✓ |
| `handleGetProject` | `ProjectHandler.Get` | ✓ |
| `handleAddProject` | `ProjectHandler.Create` | ✓ |
| `handleUpdateProject` | `ProjectHandler.Update` | ✓ |
| `handleStartScan` | `ScanHandler.StartBaseline` | ✓ |
| `handleStartIntegrityScan` | `ScanHandler.StartIntegrity` | ✓ |
| `handleCancelJob` | `ScanHandler.Cancel` | ✓ |
| `handleGetFIMEvents` | `FIMHandler.GetEvents` | ✓ |
| `handleStartFIMWatcher` | `FIMHandler.StartWatcher` | ✓ |
| `handleStopFIMWatcher` | `FIMHandler.StopWatcher` | ✓ |

### ⚠️ Butuh Use Case Baru

| Legacy Handler | Deskripsi | Action |
|----------------|-----------|--------|
| `handleGetProjectJobs` | List jobs untuk project | Buat `job.UseCase`, `JobHandler` |
| `handleGetProjectFiles` | List semua file project | Buat `file.UseCase`, `FileHandler` |
| `handleGetProjectFilesPaginated` | List file dengan pagination | Extend `FileHandler` |
| `handleGetOrphanFiles` | List orphan files | Extend `FileHandler` |
| `handleGetFileDetail` | Detail satu file | Extend `FileHandler` |
| `handleGetFileOJSRelations` | Relasi file ke OJS entities | OJS template handler |
| `handleGetFIMStats` | FIM statistics | Extend `FIMHandler` |
| `handleGetFIMEventStats` | Event statistics | Extend `FIMHandler` |
| `handleGetFIMWatcherStatus` | Watcher status | Extend `FIMHandler` |
| `handleAuditProject` | FIM audit report | Extend `FIMHandler` |
| `handleGetProjectDetails` | OJS project details | OJS template handler |
| `handleTestConnection` | Test DB connection | OJS template handler |
| `handleResetBaseline` | Reset baseline | Extend `ScanHandler` |

## Grouping untuk Migration

### Group 1: Auth (1 handler - masih di legacy)
- `handleLogin` → perlu dihapus dari legacy
- `handleGetLogs` → perlu dihapus dari legacy

### Group 2: Project CRUD (4 handlers - sudah clean)
- `handleGetProjects` → perlu dihapus dari legacy
- `handleGetProject` → perlu dihapus dari legacy
- `handleAddProject` → perlu dihapus dari legacy
- `handleUpdateProject` → perlu dihapus dari legacy

### Group 3: Scan/Jobs (5 handlers - sebagian clean)
- `handleStartScan` → sudah clean
- `handleResetBaseline` → perlu extend ScanHandler
- `handleStartIntegrityScan` → sudah clean
- `handleGetProjectJobs` → perlu JobHandler baru
- `handleCancelJob` → sudah clean

### Group 4: Files (5 handlers - perlu FileHandler baru)
- `handleGetProjectFiles`
- `handleGetProjectFilesPaginated`
- `handleGetOrphanFiles`
- `handleGetFileDetail`
- `handleGetFileOJSRelations`

### Group 5: FIM/Events (5 handlers - sebagian clean)
- `handleGetFIMEvents` → sudah clean
- `handleGetFIMStats` → perlu extend
- `handleGetFIMEventStats` → perlu extend
- `handleAuditProject` → perlu extend
- `handleGetProjectDetails` → perlu OJS handler

### Group 6: Watcher (3 handlers - sudah clean)
- `handleStartFIMWatcher` → sudah clean
- `handleStopFIMWatcher` → sudah clean
- `handleGetFIMWatcherStatus` → perlu extend

## Migration Order

1. **Phase A**: Tambahkan JobHandler + job.UseCase (handleGetProjectJobs, handleCancelJob)
2. **Phase B**: Tambahkan FileHandler + file.UseCase (semua file handlers)
3. **Phase C**: Extend FIMHandler dengan stats/event methods
4. **Phase D**: OJS template handlers untuk details/test-connection
5. **Phase E**: Hapus legacy handlers dari main.go
6. **Phase F**: Hapus handlers.go

## Dependencies

```
handlers.go
    ↓
    ├── use cases/
    │   ├── project (ada)
    │   ├── scan (ada)
    │   ├── fim (ada)
    │   ├── job (BUTUH)
    │   └── file (BUTUH)
    │
    ├── handlers/
    │   ├── project.go (ada)
    │   ├── scan.go (ada)
    │   ├── fim.go (ada)
    │   ├── auth.go (ada)
    │   ├── job.go (BUTUH)
    │   └── file.go (BUTUH)
    │
    └── templates/
        └── ojs/ (ada)
```

## Backward Compatibility

main.go saat ini mount BOTH:
- Clean Architecture router (/)
- Legacy handlers (/api)

Target: Hapus legacy routes, gunakan clean architecture saja.

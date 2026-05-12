# GPU Booking Console Plugin

[![Tests](https://github.com/rhai-code/gpu-booking-app-plugin/actions/workflows/test.yaml/badge.svg)](https://github.com/rhai-code/gpu-booking-app-plugin/actions/workflows/test.yaml)

An OpenShift Console Dynamic Plugin for booking and managing shared GPU resources with [Kueue](https://kueue.sigs.k8s.io/) integration. Users reserve GPU time slots through an interactive calendar interface embedded directly in the OpenShift console, and the system automatically manages Kueue ClusterQueue quotas to enforce reservation priority via workload preemption.

## Features

- **OpenShift Console integration** - runs as a dynamic plugin inside the OpenShift web console (admin perspective)
- **Interactive calendar** with day/multi-day/hour-range bookings, Ctrl+click multi-select, Shift+click ranges, and right-click context menu
- **GPU auto-discovery** - automatically detects GPU types, MIG partitions, and cluster capacity from Kubernetes node labels and allocatable resources
- **Kueue integration** - automatically syncs LocalQueue GPU usage as "consumed" bookings; user reservations take priority and trigger workload preemption
- **Reservation system** - creates per-user ClusterQueues with protected `nominalQuota`, ensuring reserved workloads cannot be preempted by unreserved ones
- **Admin dashboard** - sortable/filterable bookings table, runtime reservation sync toggle, on-demand GPU discovery, database export/import
- **Built-in help** - 8-page markdown documentation with sidebar navigation
- **OpenShift auth** - authentication via console `UserToken` proxy and Kubernetes TokenReview/SubjectAccessReview

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│ OpenShift Console                                            │
│                                                              │
│  ┌────────────────────────────────────────────────┐          │
│  │ GPU Booking Plugin (React / PatternFly v6)     │          │
│  │  BookingPage  │  AdminPage  │  HelpPage        │          │
│  └───────┬────────────────────────────────────────┘          │
│          │ /api/proxy/plugin/gpu-booking-plugin/backend/api  │
│          │ (Bearer token injected by console)                │
│  ┌───────▼────────────────────────────────────────┐          │
│  │ Console Proxy (UserToken)                      │          │
│  └───────┬────────────────────────────────────────┘          │
└──────────┼───────────────────────────────────────────────────┘
           │
  ┌────────▼─────────────────────────────────────────────┐
  │ GPU Booking Pod (:9443 TLS)                          │
  │                                                      │
  │  Go Backend                                          │
  │  ├── Auth (TokenReview + SubjectAccessReview)        │
  │  ├── Booking API (CRUD, bulk, conflict resolution)   │
  │  ├── Admin API (list, delete, export/import DB)      │
  │  ├── GPU Discovery (5m - auto-detect from nodes)      │
  │  ├── Kueue Sync (30s - consumed bookings)            │
  │  ├── Reservation Sync (10m - K8s resources)          │
  │  └── Static Assets (plugin dist/)                    │
  │                                                      │
  │  SQLite DB (/app/data/bookings.db)                   │
  └──────────────────────────────────────────────────────┘
           │
  ┌────────▼─────────────────────────────────────────────┐
  │ Kueue API (ClusterQueues, LocalQueues, Cohorts)      │
  │ HardwareProfiles (infrastructure.opendatahub.io)     │
  └──────────────────────────────────────────────────────┘
```

The plugin runs as a single pod with one container. The Go backend serves both the plugin's static assets (loaded by the OpenShift console) and the REST API. Authentication is handled by the console's `UserToken` proxy, which forwards the logged-in user's Bearer token to the backend.

For the full architecture details, see [ARCHITECTURE.md](ARCHITECTURE.md).

## Getting Started

### Prerequisites

- Go 1.25+
- Node.js 22+
- yarn
- SQLite development libraries (`sqlite-devel` or `sqlite-libs`)
- An OpenShift 4.x cluster with Kueue installed

### Local Development

```bash
# Install frontend dependencies
yarn install

# Start the backend (with Kueue sync disabled for local dev)
cd cmd/plugin
KUEUE_SYNC_ENABLED=false DB_PATH=./bookings.db go run .

# Start the frontend dev server (in another terminal)
yarn dev
```

The backend runs on `:9443` and the webpack dev server on `:9001` (proxying API calls to the backend).

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `9443` | Server port |
| `DB_PATH` | `/app/data/bookings.db` | SQLite database path |
| `PLUGIN_DIST_DIR` | `/app/dist` | Path to frontend static assets |
| `BOOKING_WINDOW_DAYS` | `30` | How far ahead users can book |
| `KUEUE_SYNC_ENABLED` | `true` | Enable LocalQueue watcher |
| `KUEUE_SYNC_INTERVAL` | `30` | Kueue poll interval in seconds |
| `KUEUE_BOOKING_DAYS` | `7` | Days to book ahead for consumed slots |
| `GPU_DISCOVERY_ENABLED` | `true` | Auto-discover GPUs from cluster nodes |
| `GPU_DISCOVERY_INTERVAL` | `300` | Discovery interval in seconds |

## Deployment

### Build and Push

```bash
# Build and push the container image
make podman-push

# Or build without pushing
make podman-build
```

The default image is `quay.io/eformat/gpu-booking-plugin:latest`. Override with:

```bash
make podman-push REGISTRY=my-registry.example.com REPOSITORY=my-registry.example.com/my-org/gpu-booking-plugin
```

### Deploy with Helm

```bash
helm install gpu-booking-plugin chart/ \
  -n gpu-booking-app-plugin --create-namespace
```

This creates:
- **Deployment** - single-replica pod with TLS via service-serving certificates
- **ConsolePlugin** CR - registers the plugin with OpenShift console
- **Service** - ClusterIP on port 9443
- **PVC** - 2Gi for SQLite database persistence
- **ServiceAccount + ClusterRole** - RBAC for Kueue, auth, and HardwareProfile APIs
- **Console patch** - enables the plugin in the OpenShift console

#### Key Helm Values

| Value | Default | Description |
|-------|---------|-------------|
| `image.repository` | `quay.io/eformat/gpu-booking-plugin` | Container image |
| `image.tag` | `latest` | Image tag |
| `replicas` | `1` | Pod replicas |
| `persistence.size` | `2Gi` | PVC size for SQLite |
| `tls.enabled` | `true` | Enable TLS via service-serving certs |
| `kueueSync.enabled` | `true` | Enable Kueue LocalQueue sync |
| `kueueSync.interval` | `30` | Kueue sync interval in seconds |
| `kueueSync.bookingDays` | `7` | Days ahead for consumed bookings |
| `bookingWindowDays` | `30` | Booking window in days |

### Kueue Reservation System

When Kueue sync is enabled, the plugin:

1. Polls all LocalQueues for GPU usage and creates "consumed" bookings
2. When users reserve slots, consumed bookings are evicted and per-user ClusterQueues are created with protected quotas
3. Kueue's `reclaimWithinCohort` preemption ensures reserved workloads take priority over unreserved ones
4. HardwareProfiles are created in user namespaces for OpenDataHub/RHOAI workbench scheduling
5. When reservations expire, ClusterQueues are gracefully drained (`stopPolicy: HoldAndDrain`) before deletion, giving active workloads time to complete

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full quota flow, preemption model, and sync lifecycle.

## API Endpoints

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `GET` | `/api/auth/me` | user | Current user info (username, groups, is_admin) |
| `GET` | `/api/config` | user | GPU resource specs, booking window, cluster capacity |
| `GET` | `/api/bookings` | user | List all bookings + active reservations |
| `POST` | `/api/bookings` | user | Create a booking |
| `POST` | `/api/bookings/bulk` | user | Bulk create bookings (multi-day) |
| `DELETE` | `/api/bookings?id=<id>` | user | Cancel own booking |
| `GET` | `/api/admin` | admin | All bookings (admin only) |
| `DELETE` | `/api/admin?id=<id>` | admin | Delete any booking |
| `POST` | `/api/admin/reservations` | admin | Toggle reservation sync |
| `POST` | `/api/admin/discover` | admin | Trigger GPU auto-discovery |
| `GET` | `/api/admin/database/export` | admin | Download database as JSON |
| `POST` | `/api/admin/database/import` | admin | Restore database from JSON |
| `GET` | `/api/health` | none | Health check |

## Project Structure

```
gpu-booking-app-plugin/
├── cmd/plugin/             # Go entry point
├── pkg/
│   ├── api/                # HTTP handlers (auth, bookings, admin, config)
│   ├── database/           # SQLite schema and queries
│   └── kube/               # Kueue sync + reservation management
├── src/
│   ├── components/         # React components (BookingPage, AdminPage, HelpPage, etc.)
│   ├── utils/              # AuthContext, API helpers
│   └── docs/               # Help markdown files, images, topic registry
├── chart/                  # Helm chart (deployment, ConsolePlugin, RBAC, PVC)
├── console-extensions.json # Plugin routes and navigation
├── webpack.config.ts       # Frontend build config
├── Containerfile           # Multi-stage build (Node.js + Go + UBI minimal)
└── ARCHITECTURE.md         # Detailed architecture documentation
```

## License

Apache License 2.0

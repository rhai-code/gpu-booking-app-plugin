# GPU Booking Console Plugin Architecture

This document describes the architecture of the GPU Booking OpenShift Console Dynamic Plugin, covering the frontend integration, Go backend, Kueue-based reservation system, and deployment model.

## System Overview

The GPU Booking Console Plugin is an OpenShift Console Dynamic Plugin that manages GPU resources with optional MIG (Multi-Instance GPU) partitioning. GPU types, counts, and cluster capacity are auto-discovered from Kubernetes node labels and allocatable resources. It runs as a single pod serving both a React frontend (loaded into the OpenShift console) and a Go API backend. The system combines user-facing reservations with automatic Kueue workload tracking to provide a unified view of GPU allocation.

**Key components:**
- **Console Plugin** (React/PatternFly v6 frontend) - loaded into OpenShift console via `ConsolePlugin` CR
- **Go Backend** (API server on port 9443) - booking API, Kueue sync, reservation management
- **Kueue** - Kubernetes-native job queueing with quota management
- **SQLite** - persistent booking storage (WAL mode)
- **OpenShift Console Proxy** - authentication via `UserToken` proxy type

```
+------------------------------------------------------------------+
| OpenShift Console                                                |
|                                                                  |
|  +------------------------------------------------+              |
|  | GPU Booking Plugin (React/PF6)                 |              |
|  |   BookingPage | AdminPage | HelpPage           |              |
|  +-----------+------------------------------------+              |
|              | /api/proxy/plugin/gpu-booking-plugin/backend/api  |
|              | (Bearer token injected by console)                |
|  +-----------v------------------------------------+              |
|  | Console Proxy (UserToken)                      |              |
|  +-----------+------------------------------------+              |
+--------------+---------------------------------------------------+
               |
  +------------v------------------------------------------------+
  | GPU Booking Pod (:9443 TLS)                                  |
  |                                                              |
  |  Go Backend                                                  |
  |  +-- Auth Middleware (TokenReview + SubjectAccessReview)     |
  |  +-- Booking API (CRUD, bulk, conflict resolution)           |
  |  +-- Admin API (list, delete, export/import DB)              |
  |  +-- Kueue Sync Loop (30s - consumed bookings)               |
  |  +-- Reservation Sync Loop (10m - K8s resources)             |
  |  +-- Expiry Cleaner (10m - stale ClusterQueues)              |
  |  +-- Static Asset Server (plugin dist/)                      |
  |                                                              |
  |  SQLite DB (/app/data/bookings.db)                           |
  +--------------------------------------------------------------+
```

## Frontend Architecture

The frontend is a React application using PatternFly v6 components, bundled via webpack and loaded into the OpenShift console as a dynamic plugin.

### Console Integration

**Plugin registration** (`console-extensions.json`):

| Extension Type | Details |
|---------------|---------|
| `console.navigation/section` | "GPU Booking" nav section in admin perspective |
| `console.page/route` `/gpu-booking/bookings` | Main booking page |
| `console.page/route` `/gpu-booking/admin` | Admin dashboard |
| `console.page/route` `/gpu-booking/help/:topic?` | Help documentation |
| `console.navigation/href` | Nav links: Bookings, Administration, Help |

**Exposed modules** (`package.json`):
- `BookingPage` - main booking interface
- `AdminPage` - admin dashboard
- `HelpPage` - help documentation

### Components

| Component | Purpose |
|-----------|---------|
| `BookingPage.tsx` | Main page: calendar + resource selector + booking grid + usage panel. Supports multi-select dates (Ctrl/Shift click), "My Bookings" filter, and bulk booking modal |
| `BookingGrid.tsx` | Per-resource slot grid showing reserved/consumed/available units with Reserve/Cancel/Edit/Override buttons |
| `BookingModal.tsx` | Modal for multi-day GPU reservations with date range, time range, and resource quantity selectors |
| `CalendarGrid.tsx` | Interactive month calendar with booking density indicators and date multi-select |
| `ResourceSelector.tsx` | Card gallery for GPU resource type selection (auto-discovered) with GPU equivalent weights |
| `GpuUsagePanel.tsx` | Real-time usage bar chart showing consumed/reserved/free slots per resource type with hover-to-reveal usernames |
| `PreemptionBanner.tsx` | Collapsible banner showing preempted Kueue workloads with owner, reason, message, and timestamp. Hidden when none |
| `AdminPage.tsx` | Admin: view all bookings, delete bookings, toggle Kueue sync, export/import database |
| `HelpPage.tsx` | Markdown-based help with sidebar navigation, 8 topic pages, prev/next pagination |

### Authentication Context

`AuthContext.tsx` provides user identity to all components:
1. Calls `GET /api/auth/me` with the console-injected Bearer token
2. Returns `{ username, groups, is_admin }`
3. Cached for 30 seconds client-side to pick up auth changes
4. Admin status drives visibility of admin panel and override buttons

### Routing

Uses React Router v5 (`useHistory`, `useLocation`) provided by the OpenShift console SDK. Route params are extracted from `window.location.pathname` via `useLocation` rather than `useParams`, as the console SDK routing does not reliably trigger re-renders with `useParams`.

## Backend Architecture

The Go backend serves the plugin's static assets and REST API on port 9443 with TLS (via OpenShift service-serving certificates).

### Project Structure

```
cmd/
└── plugin/main.go          # Entry point, server setup, TLS config

pkg/
├── api/
│   ├── main.go             # HTTP router, static file serving, middleware
│   ├── auth.go             # TokenReview + SubjectAccessReview, 5m cache
│   ├── bookings.go         # GET/POST/DELETE bookings, bulk booking
│   ├── admin.go            # Admin list, delete, DB export/import
│   ├── config.go           # GPU specs, booking window, cluster capacity
│   └── helpers.go          # JSON response helpers
├── database/
│   └── database.go         # SQLite schema, CRUD operations, WAL mode
└── kube/
    ├── kueue.go            # Kueue LocalQueue sync → consumed bookings
    └── reservations.go     # K8s resource sync (CQ/LQ/HardwareProfile)
```

### Authentication Flow

The console's `UserToken` proxy forwards the logged-in user's Bearer token to the backend. The auth middleware:

1. Extracts the Bearer token from the `Authorization` header
2. Validates via Kubernetes `TokenReview` API → extracts `username` and `groups`
3. Checks admin status via `SubjectAccessReview` for `gpubooking.openshift.io/bookings` with verb `admin`
4. Caches results for 5 minutes keyed by SHA256 token hash
5. Injects `username`, `groups`, and `is_admin` into the request context

**Important:** Console impersonation does NOT affect plugin proxy calls. The `UserToken` proxy always sends the logged-in user's actual token. Impersonation headers (`Impersonate-User`) are only forwarded for direct Kubernetes API calls.

### API Routes

```
GET    /api/auth/me                  → current user info (username, groups, is_admin)
GET    /api/config                   → GPU specs, booking window, cluster capacity
GET    /api/bookings                 → all bookings + active reservations + current user
POST   /api/bookings                 → create single booking (conflict resolution)
DELETE /api/bookings?id=             → cancel booking (owner or admin only)
POST   /api/bookings/bulk            → create multi-day bookings (date range + resource counts)
GET    /api/workloads/preempted      → list Kueue workloads with Preempted/Evicted conditions
GET    /api/admin                    → all bookings (admin only)
DELETE /api/admin?id=                → delete booking (admin only)
POST   /api/admin/reservations       → toggle Kueue sync (admin only)
GET    /api/admin/database/export    → download database JSON (admin only)
POST   /api/admin/database/import    → restore database (admin only)
GET    /api/health                   → health check + namespace
```

### Username Sanitization

The console plugin receives full email addresses from `TokenReview` (e.g. `cluster-admin@redhat.com`), whereas the original standalone app received short usernames from OAuth proxy `X-Forwarded-User`. Kubernetes resource names must be valid RFC 1123 DNS labels, so:

- `@` and domain are stripped: `cluster-admin@redhat.com` → `mhepburn`
- Invalid characters replaced with `-`
- Result trimmed of leading/trailing `-` and `.`
- Truncated to 253 characters

This ensures `user-mhepburn` namespaces and ClusterQueues match the existing convention.

## GPU Resource Pool

GPU resources are auto-discovered from the cluster by querying Kubernetes nodes with the `nvidia.com/gpu.present=true` label. The discovery module (`pkg/kube/discovery.go`) extracts:

- **GPU product and memory** from node labels (`nvidia.com/gpu.product`, `nvidia.com/gpu.memory`)
- **GPU and MIG counts** from `status.allocatable` resources (`nvidia.com/gpu`, `nvidia.com/mig-*`)
- **Kueue ResourceFlavor names** from the Kueue API (used for reservation ClusterQueue flavors)
- **Total CPU and memory** summed across all GPU nodes

GPU equivalent weights for MIG slices are calculated automatically as `mig_memory / full_gpu_memory`. The `share` field (fraction of total CPU/memory per unit) is derived from `gpuEquivalent / total_equivalent_units`.

Discovery runs at startup (synchronously, before Kueue/reservation sync) and then periodically (default: every 5 minutes). The configuration is stored in a thread-safe `atomic.Pointer[GPUConfig]` so concurrent readers always see a consistent snapshot. Administrators can trigger immediate re-discovery via `POST /api/admin/discover`.

Discovery assumes a **homogeneous GPU cluster** — all GPU nodes have the same GPU product. In heterogeneous clusters (e.g., mixed A100 and H100 nodes), all `nvidia.com/gpu` resources are aggregated into a single "Full GPU" entry using the product name from the first node returned by the API. MIG gpuEquivalent ratios are calculated against that first node's GPU memory. Per-product grouping is not currently supported.

When auto-discovery is disabled (`GPU_DISCOVERY_ENABLED=false`), the backend falls back to a static ConfigMap (`gpu-booking-plugin-gpu-config`) mounted at `/app/config/gpu-config.json`, or built-in defaults if neither is available. The static config path supports heterogeneous clusters — you can define multiple full GPU entries (e.g., A100 and H100) with independent names, counts, shares, and gpuEquivalent weights in the same config file.

## Kueue Resource Hierarchy

All Kueue resources share a single flat Cohort. User reservations carve out protected quota from the shared pool.

![Kueue Resource Hierarchy](images/kueue-resource-hierarchy.png)

### Resource Relationships

When a user has an active reservation, three Kubernetes resources are created:

1. **ClusterQueue** (`user-<username>`) - joins the `unreserved` cohort, scoped to the user's namespace. Holds the reserved `nominalQuota` for CPU, memory, and GPU resources. Labeled with `rhai-tmm.dev/until` for expiry tracking. On expiry, the ClusterQueue is gracefully drained via `stopPolicy: HoldAndDrain` before deletion (see Expiration Cleaner below).

2. **LocalQueue** (`reserved`) - lives in the `user-<username>` namespace and points to the user's ClusterQueue. This is the queue users submit workloads to.

3. **HardwareProfile(s)** - one per GPU resource type reserved (e.g. `reserved-gpu`, `reserved-mig-35gb`). Lives in the user's namespace and references the `reserved` LocalQueue. Uses API group `infrastructure.opendatahub.io`. Provides the scheduling interface for OpenDataHub/RHOAI workbenches.

### Quota Flow

```
Total GPU Pool (values.yaml: totalResources)
    |
    |- User reservations subtracted (per-user nominalQuota)
    |
    v
Remaining = Cohort nominalQuota (shared pool)
    |
    |- ClusterQueue: unreserved (nominalQuota: 0, borrows from Cohort)
    |- ClusterQueue: unreserved-priority (nominalQuota: 0, borrows from Cohort)
    |- ClusterQueue: user-alice (nominalQuota: reserved amount)
    |- ClusterQueue: user-bob (nominalQuota: reserved amount)
```

## Preemption Model

The preemption design ensures user reservations are pre-eminent over unreserved workloads.

![Preemption Model](images/preemption-model.png)

| Queue Type | `reclaimWithinCohort` | `borrowWithinCohort` | Effect |
|---|---|---|---|
| `user-<name>` | `Any` | `Never` | Can reclaim quota from borrowing workloads; cannot borrow beyond reservation |
| `unreserved` | (none) | (none) | Cannot preempt anyone; all workloads are borrowing |
| `unreserved-priority` | (none) | `LowerPriority` (threshold 100) | Can preempt low-priority borrowing workloads only |

**Key guarantees:**
- Workloads within a user's `nominalQuota` are **never preemptible** (they are not "borrowing")
- User CQs can **reclaim** from any workload borrowing from the Cohort (all unreserved workloads)
- Unreserved CQs **cannot preempt** user workloads because they have no reclaim policy
- Beyond their reservation, users compete fairly with no preemption rights

## Consumed vs Reserved Bookings

The system tracks two types of bookings that interact through a priority-based conflict resolution model.

![Consumed vs Reserved Booking Flow](images/consumed-vs-reserved-booking-flow.png)

| Property | Reserved | Consumed |
|----------|----------|----------|
| **Source** | `"reserved"` | `"consumed"` |
| **Created by** | Users via booking UI | Kueue sync daemon |
| **Evictable** | No (blocks other reservations) | Yes (evicted by reserved bookings) |
| **Cancellable** | Yes (by owner or admin) | No (admin only; repopulated on next sync) |
| **ID format** | UUID | `kueue-{namespace}-{resource}-s{slot}-{date}` |
| **Triggers K8s sync** | Yes (creates ClusterQueue/LocalQueue/HardwareProfile) | No (reflects existing K8s state) |

### Conflict Resolution Rules

1. **Empty slot** - booking proceeds normally
2. **Slot occupied by `consumed`** - consumed booking is evicted, reserved booking takes its place. This reduces the unreserved Cohort and triggers Kueue workload preemption.
3. **Slot occupied by `reserved`** - returns `409 Conflict` (`slot_taken`)
4. **Database uniqueness** - `UNIQUE(resource, slot_index, date, slot_type)` constraint prevents exact duplicates

## Sync Lifecycle

Three independent sync loops run concurrently in the Go backend.

![Sync Lifecycle](images/sync-lifecycle.png)

### 1. Kueue Sync Loop (every 30s)

**File:** `pkg/kube/kueue.go`
**Controlled by:** `KUEUE_SYNC_ENABLED`, `KUEUE_SYNC_INTERVAL`

Polls all LocalQueues in the cluster and creates `consumed` bookings reflecting actual GPU usage:

- Lists LocalQueues via `kueue.x-k8s.io/v1beta1` API
- Filters for queues with active workloads (`reservingWorkloads > 0` or `admittedWorkloads > 0`)
- Reads `flavorUsage` to determine GPU resource counts per namespace
- Aggregates across multiple LocalQueues in the same namespace (e.g. `default`, `unreserved`, `unreserved-priority`)
- Assigns globally unique slot indices per resource to avoid UNIQUE constraint collisions
- Resolves namespace `rhai-tmm.dev/owner` label as booking owner (falls back to namespace name)
- Books from today through `KUEUE_BOOKING_DAYS` days ahead
- Reconciles: adds missing bookings, removes stale future bookings, skips slots already reserved
- Historical bookings (past dates) are preserved

### 2. Reservation Sync Loop (every 10min)

**File:** `pkg/kube/reservations.go`
**Controlled by:** Runtime toggle via `POST /api/admin/reservations`

Materializes Kueue resources (ClusterQueue, LocalQueue, HardwareProfile) for today's reserved bookings:

- Queries today's and tomorrow's `reserved` bookings from SQLite
- Groups by user, sums GPU counts per resource type
- Calculates CPU/memory allocation from GPU share ratios
- Determines `until` timestamp from the latest `end_hour`
- Applies ClusterQueue, LocalQueue, and HardwareProfile(s) via server-side apply
- Updates the `unreserved` Cohort's `nominalQuota` with remaining resources

### 3. Expiration Cleaner (every 10min)

**File:** `pkg/kube/reservations.go`

Cleans up expired Kueue resources using a graceful two-phase drain process, following the [Kueue ClusterQueue deletion procedure](https://kueue.sigs.k8s.io/docs/tasks/troubleshooting/troubleshooting_delete_clusterqueue):

- Lists all ClusterQueues with the `rhai-tmm.dev/until` label
- Also removes resources for users who no longer have active bookings (`removeStaleReservations`)

**Phase 1 — Drain:** When a ClusterQueue expires or becomes stale and has active workloads:
- Sets `spec.stopPolicy: HoldAndDrain` on the ClusterQueue (stops new admissions, evicts admitted workloads)
- Adds labels `rhai-tmm.dev/draining: "true"` and `rhai-tmm.dev/drain-start: <unix-timestamp>`
- Immediately deletes the associated LocalQueue and HardwareProfiles (prevents new workload submissions)

**Phase 2 — Delete:** On subsequent cleaner cycles, draining ClusterQueues are checked:
- If `admittedWorkloads + pendingWorkloads + reservingWorkloads == 0` → ClusterQueue is deleted
- If the drain timeout (30 minutes) has elapsed → ClusterQueue is force-deleted with a warning log
- Otherwise, the queue remains in draining state until the next cycle

If the ClusterQueue has **no active workloads** at expiry time, it is deleted immediately (skipping the drain phase).

The Cohort `nominalQuota` is re-synced after cleanup.

### Async Sync Triggers

In addition to the periodic loops, reservation sync is triggered asynchronously (via `go syncReservations()`) after:

- Creating a booking (`POST /api/bookings`)
- Deleting a booking (`DELETE /api/bookings`)
- Admin deleting a booking (`DELETE /api/admin`)
- Bulk booking creation (`POST /api/bookings/bulk`)
- Database import (`POST /api/admin/database/import`)

## Deployment Architecture

The application deploys as a single Kubernetes pod with one container, managed by a Helm chart in `chart/`.

### Container Build (Multi-stage)

```
Stage 1: frontend-builder (UBI9 Node.js 22)
  └── yarn build → dist/ (webpack + ConsoleRemotePlugin)

Stage 2: backend-builder (UBI9 Go 1.25)
  └── CGO_ENABLED=1 go build → /backend (SQLite requires CGO)

Stage 3: final (UBI9 ubi-minimal)
  ├── /app/backend (Go binary)
  ├── /app/dist/ (plugin static assets)
  ├── /app/data/bookings.db (SQLite, PVC-backed)
  ├── /app/config/gpu-config.json (ConfigMap-mounted GPU config)
  └── Port 9443 (TLS via service-serving certs)
```

### Kubernetes Resources

| Resource | Details |
|----------|---------|
| **Deployment** | Single replica, non-root (UID 1001), read-only rootfs, liveness/readiness on `/api/health` |
| **ConsolePlugin** CR | Registers plugin with OpenShift console, `UserToken` proxy type, service backend |
| **Service** | ClusterIP on port 9443, `service.beta.openshift.io/serving-cert-secret-name` annotation |
| **ConfigMap** | GPU resource configuration (`gpu-config.json`), mounted at `/app/config` |
| **PVC** | 2Gi for SQLite database persistence |
| **ServiceAccount** | With RBAC for Kueue, namespace, auth, and HardwareProfile APIs |
| **ClusterRole** | `localqueues`, `clusterqueues`, `cohorts` (Kueue); `namespaces`, `tokenreviews`, `subjectaccessreviews` (K8s); `hardwareprofiles` (OpenDataHub) |
| **Console Patch** | Enables plugin in the `Console` operator config |

### RBAC

The plugin's ServiceAccount requires:

| API Group | Resources | Verbs |
|-----------|-----------|-------|
| `kueue.x-k8s.io` | `clusterqueues`, `clusterqueues/status` | get, list, create, patch, delete |
| `kueue.x-k8s.io` | `localqueues` | list, create, patch, delete |
| `kueue.x-k8s.io` | `workloads` | list |
| `kueue.x-k8s.io` | `cohorts` | create, patch |
| `""` (core) | `namespaces` | get |
| `authentication.k8s.io` | `tokenreviews` | create |
| `authorization.k8s.io` | `subjectaccessreviews` | create |
| `infrastructure.opendatahub.io` | `hardwareprofiles` | list, create, patch, delete |

Admin access for users is controlled by a separate ClusterRole that grants the `admin` verb on `gpubooking.openshift.io/bookings`. This custom RBAC rule is checked via SubjectAccessReview and is not included in the standard OpenShift `admin` ClusterRole, so project owners do not automatically get booking admin access.

## Help System

The plugin includes a built-in help system with 8 markdown documentation pages:

| Topic | Slug |
|-------|------|
| Getting Started | `getting-started` |
| Calendar & Navigation | `calendar` |
| Making Bookings | `making-bookings` |
| GPU Resources | `gpu-resources` |
| Kueue & Auto-Bookings | `kueue` |
| Admin Dashboard | `admin` |
| Slots & Conflicts | `slots-and-conflicts` |
| FAQ | `faq` |

Markdown files are imported as strings via webpack `asset/source` rule and rendered at runtime with `react-markdown`, `remark-gfm`, and `rehype-raw`. Images are imported as webpack assets and resolved via an `imageMap` registry.

## Testing

The project has unit tests for both backend and frontend. All test commands are available via Makefile targets.

### Running Tests

| Command | Description |
|---------|-------------|
| `make test` | Run all Go + frontend tests |
| `make test-go` | Run Go tests only |
| `make test-frontend` | Run frontend tests only (`vitest`) |
| `make coverage` | Run all tests with coverage reports |
| `make coverage-go` | Go coverage → `coverage-go/coverage.html` |
| `make coverage-frontend` | Frontend coverage → `coverage/index.html` |

### Backend (Go)

Tests use the standard `testing` package with `net/http/httptest` for handler tests and temporary SQLite databases for isolation. Each handler test creates a fresh database via `t.TempDir()` and injects user context to simulate authenticated requests without requiring a Kubernetes cluster.

Test files:

| File | Covers |
|------|--------|
| `pkg/api/validate_test.go` | Booking ID and date validation |
| `pkg/api/helpers_test.go` | JSON response helpers |
| `pkg/api/bookings_test.go` | Booking CRUD, conflict resolution, bulk operations, consumed eviction |
| `pkg/api/admin_test.go` | Admin list/delete/toggle, pagination, filters, DB export/import |
| `pkg/api/auth_test.go` | User context extraction, MeHandler, AuthMiddleware (DevMode, cache, health bypass) |
| `pkg/api/config_test.go` | Config endpoint |
| `pkg/api/ratelimit_test.go` | Per-IP rate limiting and burst behavior |
| `pkg/database/database_test.go` | Init, schema, CRUD, unique constraints, config loading |

**Note:** `pkg/kube/` is not unit-tested as it requires a live Kubernetes cluster with Kueue installed.

### Frontend (TypeScript)

Tests use [Vitest](https://vitest.dev/) with `@testing-library/react-hooks` for hook tests and `vi.mock`/`vi.fn` for API mocking.

Test files:

| File | Covers |
|------|--------|
| `src/utils/constants.test.ts` | Date formatting, weekend detection, month/date range utilities, GPU equivalent calculations |
| `src/utils/api.test.ts` | All API client functions with mocked `fetch`, CSRF token handling, error responses |
| `src/utils/hooks.test.ts` | `useBookings`, `useConfig`, `usePreemptedWorkloads`, `useClock` — mount, polling, cleanup, error handling |

## Key Files

| File | Purpose |
|------|---------|
| `cmd/plugin/main.go` | Entry point, TLS setup, server startup |
| `pkg/api/main.go` | HTTP router, static assets, middleware chain |
| `pkg/api/auth.go` | TokenReview + SubjectAccessReview auth |
| `pkg/api/bookings.go` | Booking CRUD, conflict resolution, bulk booking |
| `pkg/database/database.go` | SQLite schema, queries, WAL mode |
| `pkg/api/workloads.go` | Preempted workloads API handler |
| `pkg/kube/kueue.go` | Kueue LocalQueue sync → consumed bookings, preempted workload queries |
| `pkg/kube/reservations.go` | K8s resource sync (CQ/LQ/HardwareProfile), username sanitization |
| `src/components/BookingPage.tsx` | Main booking UI |
| `src/components/BookingGrid.tsx` | Per-resource slot grid |
| `src/components/HelpPage.tsx` | Help documentation viewer |
| `src/utils/AuthContext.tsx` | Auth state management with 30s TTL cache |
| `src/docs/topicRegistry.ts` | Help topic definitions and navigation structure |
| `console-extensions.json` | Plugin routes and navigation registration |
| `chart/` | Helm chart (deployment, ConsolePlugin CR, RBAC, PVC, ConfigMap, service) |

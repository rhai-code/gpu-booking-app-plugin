# Admin Dashboard

<span class="badge">Topics: Bookings Management, Source Filter, Database Export/Import, Reservation Sync</span>

---

## Overview

The admin dashboard provides a centralised view of all bookings across the cluster, with tools to manage bookings, control Kueue reservation sync, and export or import the database.

Admin access is determined by OpenShift RBAC -- users with the `gpubooking.openshift.io/bookings:admin` permission see the admin page in the console navigation. No separate login is required.

![images/admin-overview.png](images/admin-overview.png)

---

## Admin Controls

The controls card at the top of the page provides:

- **Reservation Sync toggle** -- ON/OFF switch to enable or disable Kueue reservation sync at runtime without redeploying
- **Discover GPUs** -- trigger an immediate GPU auto-discovery from the cluster, updating resource types, counts, and capacity without restarting the pod
- **Export DB** -- download the current SQLite database as a backup
- **Import DB** -- upload a replacement database file (choose file, then click Import)
- **Delete All** -- remove all bookings with confirmation

---

## Resource Filter

The resource selector cards filter the bookings table by GPU type. All resource types are selected by default. Click a card to show only that resource type; Ctrl+click (Cmd+click on Mac) to toggle individual types on or off.

The booking count updates to reflect the current filter, e.g. "18 of 129 bookings".

![images/admin-resource-filter.png](images/admin-resource-filter.png)

---

## Source Filter

The source filter buttons let you narrow bookings by their origin:

- **All** -- show all bookings (default)
- **Reserved** -- show only user-created bookings
- **Consumed** -- show only Kueue auto-bookings

Each button displays a count badge showing how many bookings match that source. The source filter works in combination with the resource filter and text filter.

---

## Text Filter

The search box in the toolbar filters by user, date, resource, source, or description. Filtering is case-insensitive and matches substrings. This works in combination with the resource selector and source filter above.

---

## Database Export / Import

![images/admin-filters-database.png](images/admin-filters-database.png)

### Export

Click **Export DB** to download the current `bookings.db` file. The server flushes the WAL (Write-Ahead Log) before streaming the file, ensuring a consistent snapshot.

### Import

Use the file chooser to select a replacement database file, then click **Import**. Accepted formats: `.db`, `.sqlite`, `.sqlite3` (max 100MB).

<div class="alert alert-warning">
  <strong>Warning</strong>
  <p>Importing a database replaces all existing bookings. Export a backup first if you want to preserve the current data.</p>
</div>

---

## Bookings Table

The main table lists all bookings with sortable columns:

| Column | Description |
|--------|-------------|
| **ID** | Unique booking identifier (deterministic for Kueue bookings, random for user bookings) |
| **User** | The booking owner (OpenShift username or namespace name for Kueue bookings) |
| **Resource** | GPU resource type (e.g. `nvidia.com/gpu`, `nvidia.com/mig-*`) |
| **Slot** | Slot index (0-based) |
| **Date** | Booking date (YYYY-MM-DD) |
| **Source** | `reserved` (user) or `consumed` (Kueue), colour-coded |
| **Created** | Timestamp when the booking was created |
| **Actions** | Delete button with confirmation |

Click any sortable column header to sort ascending; click again to reverse.

![images/admin-bookings-table.png](images/admin-bookings-table.png)

---

## Deleting Bookings

### Single Booking

Click the **Delete** button on any row. A confirmation prompt appears with **Confirm** and **Cancel** buttons. Admin can delete any booking, including consumed Kueue bookings.

### Delete All

Click **Delete All** in the controls card. A confirmation modal appears before deletion proceeds. After deletion, consumed bookings will be repopulated on the next Kueue sync cycle.

---

## Reservation Sync Toggle

The **Reservation Sync** switch in the controls card controls whether the server syncs Kueue LocalQueue usage and manages per-user ClusterQueues at runtime.

- **ON** -- the server polls LocalQueues, creates consumed bookings, and manages reservation ClusterQueues
- **OFF** -- sync and reservation management are paused; existing bookings and ClusterQueues are not affected

This is useful for maintenance windows or debugging without needing to redeploy.

---

## Auto-Refresh

The admin dashboard automatically refreshes every 30 seconds to show the latest bookings and sync state. You can also click **Refresh** at any time.

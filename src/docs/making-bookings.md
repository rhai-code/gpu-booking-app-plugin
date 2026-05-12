# Making Bookings

<span class="badge">Topics: Book GPU, Edit, Cancel, Bulk Cancel, Override</span>

## Creating a Booking

You can book GPU resources directly from the calendar. You can book multiple resources across multiple days.

To book GPU resources, right-click a date in the calendar and select **Book GPU** from the context menu.

![images/make-booking.png](images/make-booking.png)

This opens the booking dialog.

<div class="alert alert-info">
  <strong>Tip</strong>
  <p>Select multiple dates first (Ctrl+click or Shift+click in the calendar), then right-click to open the dialog — the date range will be pre-filled from your selection.</p>
</div>

### Booking dialog fields

![images/booking-dialog.png](images/booking-dialog.png)

| Field | Description |
|-------|-------------|
| **Date Range** | Start and end dates for the booking. Pre-filled from your calendar selection, can be overridden. |
| **Hours** | Start and end hours in your local timezone. Defaults to full day (00:00 to 00:00 +1d). The UTC equivalent is shown below when not a full day. |
| **Resources** | +/- selectors for each GPU/MIG resource type. Shows available units and GPU equivalent weight per type. |
| **Description** | Free text field (max 160 characters) for noting the purpose of the booking (e.g., model training, inference testing). |

### GPU equivalents

The dialog shows a running total of GPU equivalents for your selection. A full GPU = 1.0 equivalent. MIG partitions are fractional, calculated from their memory relative to the full GPU.

![images/gpu-equivalents.png](images/gpu-equivalents.png)

### Availability

Each resource type shows how many units are available across your selected date range. Only reserved bookings count against availability — consumed (Kueue) bookings are shown but can be overridden and do not block your booking.

### Submitting

Click the **Book** button to create all bookings at once. The system auto-finds available slot indices for each resource, date, and hour range. Consumed bookings are automatically evicted to make room.

---

## Editing a Booking

To modify an existing reservation:

1. Hover over your booking in the grid to reveal the **Edit** button
2. Click **Edit** to open the booking dialog pre-filled with the existing booking details
3. Adjust the dates, hours, resources, or description as needed
4. Click **Save Changes**

Under the hood, editing cancels the original booking and creates a new one. This allows you to change any aspect of the booking including the resource type, dates, and hours.

![images/edit-button.png](images/edit-button.png)

---

## Cancelling a Booking

To cancel one of your bookings:

1. Hover over your booking in the grid to reveal the **Cancel** button
2. Click **Cancel**, then click **Confirm** to proceed

![images/cancel-button.png](images/cancel-button.png)

You can only cancel your own reserved bookings. If you need to cancel someone else's booking, contact an administrator.

<div class="alert alert-warning">
  <strong>Note</strong>
  <p>You cannot cancel bookings on past dates. Historical bookings are read-only.</p>
</div>

---

## Bulk Cancel (Cancel All)

When you have many bookings across multiple dates and resource types, you can cancel them all at once using the **Cancel All** button.

### How to use

1. Click **My Bookings** in the toolbar — this shows all your reserved bookings and auto-selects the resource types you have bookings for

![images/my-bookings-cancel-all.png](images/my-bookings-cancel-all.png)

2. The **Cancel All (N)** button appears showing the total count of your reserved bookings across all selected resource types

### Filtering by resource type

If you only want to cancel bookings for specific resource types, use the **resource selector** to deselect the types you want to keep. The Cancel All count updates automatically to reflect only the selected resources.

For example, if you booked both full GPU and MIG resources but only want to cancel the MIG bookings, deselect the full GPU card from the resource selector.

![images/cancel-all-filtered.png](images/cancel-all-filtered.png)

### Confirmation

Clicking **Cancel All** shows a confirmation prompt. Click **Confirm** to proceed or **No** to go back.

![images/cancel-all-confirm.png](images/cancel-all-confirm.png)

<div class="alert alert-info">
  <strong>Tip</strong>
  <p>Bulk cancel only affects your own reserved bookings. Consumed (Kueue) bookings and other users' bookings are not affected.</p>
</div>

---

## Overriding Kueue Bookings

When Kueue auto-creates bookings based on active GPU workloads, those slots show a lightning bolt label and an amber **Override** button.

![images/kueue-override.png](images/kueue-override.png)

Clicking **Override** will:

1. Remove the Kueue auto-booking
2. Create a reserved booking in its place under your username
3. The Kueue workload may be preempted to free up the resource for you (this happens once you use the resource you booked)

Reserved bookings always take priority over consumed bookings. This is useful when you need guaranteed access to a resource that is currently in use by an unreserved workload.

<div class="alert alert-info">
  <strong>How preemption works</strong>
  <p>Your reservation reduces the Cohort's available quota. Kueue will preempt the lowest-priority borrowing workload to honour your reservation. The workload will be re-queued and can resume when resources become available. Any preempted workloads will appear in the collapsible banner below the GPU Usage Overview.</p>
</div>

---

## Active Reservations

When your booking has an active Kubernetes reservation (ClusterQueue + LocalQueue created for your quota), the booking cell shows a snake indicator with your namespace, e.g. `user-jsmith`.

This means your reserved quota is live in the cluster and Kueue will protect your workloads from preemption within that allocation.

---

## Auto-Refresh

The booking grid automatically refreshes every 30 seconds to show the latest bookings. You can also click the **Refresh** button in the header at any time.

---

## Error Handling

If a booking operation fails, an error banner appears at the top of the page. Common errors:

| Error | Meaning |
|-------|---------|
| `slot_taken` | Another user reserved this slot while you were viewing the page. Refresh and try a different slot. |
| `no_slots_available` | Not enough free units for the requested resource/date/hour combination. Reduce the count or try different dates. |
| `consumed_booking` | You tried to cancel a Kueue auto-booking. Use the Override button instead, or contact an admin. |
| Network error | The server may be temporarily unavailable. Try refreshing the page. |

---

## Next Steps

- [GPU Resources](gpu-resources) -- understand the different resource types and MIG partitions
- [Slot Types & Conflicts](slots-and-conflicts) -- learn how booking conflicts work

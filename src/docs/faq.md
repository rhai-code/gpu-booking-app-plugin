# FAQ

<span class="badge">Topics: Questions, Troubleshooting</span>

---

## General

### How far ahead can I book?

The booking window is configurable (default: 30 days from today). You can see available dates in the calendar -- greyed-out dates are outside the booking window.

### Can I book weekends?

Yes. Weekend dates are shown with a lighter background in the calendar but are fully bookable.

### Can I book specific hours instead of a full day?

Yes. Open the booking modal (right-click a date and select **Book GPU**) and adjust the start and end hours. Hours are shown in your local timezone; they are converted to UTC for storage. The default is full day.

### Can I book multiple resource types at once?

Yes. In the booking modal, use the +/- buttons to add multiple GPU and MIG resource types in a single booking. Each resource type will have its own slot indices allocated automatically.

### How do I see multiple resource types at once?

Ctrl+click (or Cmd+click on Mac) the resource cards in the header to select multiple types. A separate booking grid is shown for each selected resource.

### I can't see a colleague's booking for today

Bookings are stored as date strings (e.g. `2026-04-07`). If your colleague is in a different timezone, their "today" may be a different calendar date to yours. The GPU Usage Overview automatically combines both your local date and the UTC date when they differ, so you should see all current bookings there. You can also click the adjacent date in the calendar to check.

### Can I see past bookings?

Yes. Navigate back to previous months using the calendar arrows. Past dates show as read-only with a **HISTORY** badge.

### Where can I find all my bookings?

Click the **My Bookings** button in the header. It selects all dates with your reserved bookings and navigates to the earliest month. Click it again to return to today.

---

## Booking Issues

### I got a "slot_taken" error

Another user reserved the same slot while you were viewing the page. Click **Refresh** to see the latest state and try a different slot or date.

### I got a "no_slots_available" error

There are not enough free units for the resource type, date range, and hour range you requested. Try reducing the count, choosing different dates, or using a different resource type.

### I cannot cancel a booking

There are a few reasons this can happen:

- **Past booking** -- you cannot cancel bookings on past dates
- **Not your booking** -- you can only cancel bookings you created
- **Consumed booking** -- auto-bookings from Kueue cannot be cancelled by normal users. Use the **Override** button to replace it with your own reserved booking, or ask an administrator

### How do I cancel all my bookings at once?

Click **My Bookings**, then click the **Cancel All (N)** button that appears. To cancel only specific resource types, deselect the ones you want to keep from the resource selector before clicking Cancel All. See [Making Bookings](making-bookings) for details.

### Can I edit a booking?

Yes. Hover over your booking in the grid and click **Edit**. The booking modal opens pre-filled with the existing booking details. After saving, the old booking is cancelled and a new one is created with the updated settings.

### My booking disappeared

If a consumed auto-booking you were looking at disappeared, the underlying workload likely completed and the sync process removed the booking. Reserved bookings are never removed automatically.

---

## Kueue

### What does the lightning bolt mean?

The lightning bolt indicates a consumed auto-booking -- a slot that was automatically reserved because a GPU workload is actively running in that namespace.

### Can I override a consumed booking?

Yes. Click the amber **Override** button to replace the consumed booking with your reserved booking. Your reservation will take priority and the Kueue workload may be preempted.

### Will overriding a consumed booking stop someone's job?

It may. When you override, Kueue reduces the shared pool quota and may preempt the lowest-priority borrowing workload. The job will be re-queued and can restart when resources free up.

### How do I see which workloads have been preempted?

A collapsible banner appears between the GPU Usage Overview and the calendar whenever any workloads have been preempted. Click it to expand and see the owner, workload name, preemption reason, and timestamp for each one. It updates every 30 seconds and hides when there are no preempted workloads. See [Kueue & Auto-Bookings](kueue) for more details.

### Why do consumed bookings keep reappearing after an admin deletes them?

The sync process runs every 60 seconds. If the underlying workload is still active, the booking will be recreated. To permanently remove it, the workload itself needs to be stopped.

---

## Resources & Reservations

### What happens when I book a slot?

Two things happen:

1. A booking record is created in the database (with optional description and hour range)
2. Kubernetes resources (ClusterQueue, LocalQueue, HardwareProfile) are created to reserve GPU quota for your namespace

Your workloads submitted to the `reserved` LocalQueue are protected from preemption.

### How do I use my reservation?

Submit your workloads to the `reserved` LocalQueue in your namespace. Kueue will schedule them using your reserved quota, which is protected from preemption by unreserved workloads.

### When does my reservation expire?

Reservations expire based on the booking's end hour:
- **Full day** bookings expire at midnight UTC (00:00 next day)
- **Custom hour** bookings expire at the specified end hour (UTC)

When you have multiple bookings on the same day, the reservation lasts until the latest end hour across all your bookings. Expired Kubernetes resources are automatically cleaned up.

### What is the difference between MIG sizes?

See [GPU Resources](gpu-resources) for a detailed breakdown of the available MIG partition sizes and guidance on choosing the right one for your workload.

### The resource types changed after a cluster update

The app auto-discovers GPU resources from the cluster every 5 minutes. If the cluster's GPU hardware or MIG configuration changes (e.g., switching from H100 to L40S, or enabling/disabling MIG), the resource cards will update automatically. An administrator can also trigger immediate re-discovery from the admin dashboard using the **Discover GPUs** button.

---

## Next Steps

- [Getting Started](getting-started) -- back to basics
- [GPU Resources](gpu-resources) -- resource type details

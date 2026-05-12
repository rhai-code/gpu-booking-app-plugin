# Getting Started

<span class="badge">Topics: Overview, Login, Navigation</span>

---

## Introduction

The GPU Booking app lets you reserve GPU resources for your AI/ML workloads on OpenShift. The app automatically discovers available GPUs in the cluster -- you can book full GPUs or smaller MIG (Multi-Instance GPU) partitions for specific hours, full days, or multiple days ahead.

### What you can do

- Browse a calendar view of GPU availability across all resource types
- Reserve GPU slots with flexible date ranges and hour windows
- Multi-select GPU resource types to view multiple grids at once
- See who has booked what and when
- Edit or cancel your existing reservations
- View real-time GPU usage from Kueue workloads
- Override auto-bookings from Kueue when you need guaranteed access
- Quickly find all your bookings with the **My Bookings** button

![images/overview.png](images/overview.png)

---

## Logging In

The booking app is protected by OpenShift OAuth. When you visit the app, you will be redirected to the OpenShift login page. Log in with your OpenShift credentials.

Once authenticated, your username is automatically used for all bookings you create -- no additional sign-up required.

![images/login.png](images/login.png)

---

## Page Layout

The main page has three sections:

### Header

The dark header bar at the top shows:

- **App title** and description
- **Local clock** -- live datetime in your browser's timezone, updated every second
- **Help link** -- opens this documentation
- **My Bookings button** -- toggles to show all dates with your reservations
- **Refresh button** -- manually reload booking data

![images/header.png](images/header.png)

### Resource Selector

Below the header, GPU resource cards let you choose which resource types to display. The cards are populated automatically from the cluster's discovered GPU configuration. Click a card to select it exclusively, or **Ctrl+click** to multi-select multiple resource types. When multiple resources are selected, a separate booking grid is shown for each.

![images/resource-selector.png](images/resource-selector.png)

### Main Content Area

Below the header you will find:

1. **Mini calendar** -- month view with navigation, today indicator, and GPU usage badges
2. **GPU Usage Overview** -- visual bar chart of resource utilisation for the selected day
3. **Preempted Workloads** -- collapsible banner showing any workloads that have been preempted by Kueue (hidden when none)
4. **Booking grid(s)** -- detailed table of all slots with Reserve/Cancel/Edit buttons, one grid per selected resource type

![images/main-content.png](images/main-content.png)

---

## Next Steps

- [Calendar & Navigation](calendar) -- learn how to navigate dates and months
- [Making Bookings](making-bookings) -- reserve your first GPU slot
- [GPU Resources](gpu-resources) -- understand the different GPU types available

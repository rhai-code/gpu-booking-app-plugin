# GPU Resources

<span class="badge">Topics: GPU Types, MIG, Auto-Discovery</span>

---

## Auto-Discovery

The booking app automatically discovers GPU resources from the cluster by querying Kubernetes node labels and allocatable resources. When GPU nodes are present, the app detects:

- **GPU product name** and memory from node labels (`nvidia.com/gpu.product`, `nvidia.com/gpu.memory`)
- **Full GPU count** from node allocatable resources (`nvidia.com/gpu`)
- **MIG slice types and counts** from allocatable MIG resources (`nvidia.com/mig-*`)
- **Kueue ResourceFlavor names** for reservation quota management
- **Total CPU and memory** from GPU node allocatable capacity

The configuration updates automatically every 5 minutes. Administrators can also trigger an immediate re-discovery from the [Admin Dashboard](admin) using the **Discover GPUs** button.

---

## Available Resources

The resource cards at the top of the booking page show all GPU types discovered in your cluster. The exact types, counts, and names depend on your cluster's GPU hardware and MIG configuration.

Common resource types include:

| Resource Type | Example | Use case |
|---------------|---------|----------|
| **Full GPU** | `nvidia.com/gpu` | Large model training, full GPU inference |
| **Large MIG** | `nvidia.com/mig-3g.40gb` | Medium model fine-tuning, large inference |
| **Medium MIG** | `nvidia.com/mig-2g.20gb` | Small model training, inference workloads |
| **Small MIG** | `nvidia.com/mig-1g.10gb` | Notebooks, small experiments, development |

![images/gpu-resources.png](images/gpu-resources.png)

---

## What is MIG?

Multi-Instance GPU (MIG) lets a single physical GPU be partitioned into multiple isolated instances. Each instance has its own compute, memory, and memory bandwidth -- they behave like independent smaller GPUs.

MIG is supported on NVIDIA A100, H100, H200, B200, and GB200 GPUs. GPUs like the L40S and T4 do not support MIG and will only show as full GPU resources.

### GPU equivalents

Each resource type has a GPU equivalent weight used for capacity planning. The weight is calculated automatically based on the ratio of MIG slice memory to full GPU memory:

- **Full GPU** = 1.0
- **MIG slices** = slice_memory / full_gpu_memory

The calendar badges and booking modal show GPU equivalent totals to help you understand the overall capacity impact of your bookings.

### Choosing the right resource

- **Full GPU** -- use when your workload needs the full GPU memory or all compute cores (e.g., training large models, multi-GPU distributed training)
- **Large MIG** -- good for most single-GPU training and large inference workloads
- **Medium MIG** -- suitable for smaller model fine-tuning and inference
- **Small MIG** -- ideal for Jupyter notebooks, development, small experiments, and inference of quantised models

<div class="alert alert-info">
  <strong>Tip</strong>
  <p>Start with the smallest partition that fits your workload. You can always book a larger resource if needed. Smaller partitions leave more capacity available for other users.</p>
</div>

---

## Resource Selector

Switch between resource types using the cards in the header.

![images/resource-selector-detail.png](images/resource-selector-detail.png)

### Multi-select

You can view multiple resource types simultaneously:

- **Click** a card to select it exclusively
- **Ctrl+click** (or Cmd+click on Mac) to add or remove a resource type from the selection
- At least one resource type must remain selected

When multiple resources are selected, the booking grid shows a separate table for each resource type, each with its own unit columns and availability counts.

![images/gpu-resources-multiple.png](images/gpu-resources-multiple.png)

### Default selection

The app starts with the first discovered GPU type selected. You can Ctrl+click to add additional resources alongside it.

---

## Cluster Resources

Each booking reservation translates into Kueue ClusterQueue resources in the OpenShift cluster. When you have an active booking:

- A **ClusterQueue** is created with your reserved GPU quota (plus proportional CPU and memory)
- A **LocalQueue** in your namespace points to your ClusterQueue
- **HardwareProfile(s)** are created for each GPU resource type you reserved

Your workloads submitted to the `reserved` LocalQueue are protected from preemption by unreserved workloads.

### Reservation expiry

Reservations use the `rhai-tmm.dev/until` label with a UTC timestamp derived from the latest `end_hour` across your bookings for the day. Full day bookings expire at midnight UTC (00:00 next day). Hourly bookings expire at their specified end hour.

---

## Next Steps

- [Making Bookings](making-bookings) -- reserve GPU slots
- [Kueue & Auto-Bookings](kueue) -- how automatic bookings from workloads work

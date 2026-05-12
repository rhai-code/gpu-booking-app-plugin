import { Booking, GPUResource } from './constants';

const PLUGIN_NAME = 'gpu-booking-plugin';
const PROXY_BASE = `/api/proxy/plugin/${PLUGIN_NAME}/backend/api`;

function getCSRFToken(): string {
  const match = document.cookie.match(/(?:^|;\s*)csrf-token=([^;]*)/);
  return match ? match[1] : '';
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'X-CSRFToken': getCSRFToken(),
    ...(options?.headers as Record<string, string> | undefined),
  };
  const resp = await fetch(`${PROXY_BASE}${path}`, {
    ...options,
    headers,
  });
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || resp.statusText);
  }
  return resp.json();
}

// Auth
export interface AuthInfo {
  username: string;
  groups: string[];
  is_admin: boolean;
}

export const getAuthInfo = () => request<AuthInfo>('/auth/me');

// Config
export interface ConfigResponse {
  resources: GPUResource[];
  bookingWindowDays: number;
  totalCpu: number;
  totalMemory: number;
}

export const getConfig = () => request<ConfigResponse>('/config');

// Bookings
export interface BookingsResponse {
  bookings: Booking[];
  activeReservations: Record<string, string>;
  currentUser: string;
}

export const getBookings = () => request<BookingsResponse>('/bookings');

export interface BookingRequest {
  resource: string;
  slotIndex: number;
  date: string;
  slotType: string;
  description?: string;
  startHour?: number;
  endHour?: number;
  utcOffset?: number;
}

export const createBooking = (data: BookingRequest) =>
  request<Booking>('/bookings', { method: 'POST', body: JSON.stringify(data) });

export interface BulkBookingRequest {
  resources: Record<string, number>;
  startDate: string;
  endDate: string;
  description: string;
  startHour: number;
  endHour: number;
  utcOffset: number;
}

export interface BulkBookingResponse {
  bookings: Booking[];
  errors: string[];
}

export const createBulkBooking = (data: BulkBookingRequest) =>
  request<BulkBookingResponse>('/bookings/bulk', { method: 'POST', body: JSON.stringify(data) });

export const cancelBooking = (id: string) =>
  request<{ status: string }>(`/bookings?id=${encodeURIComponent(id)}`, { method: 'DELETE' });

export interface BulkCancelResponse {
  deleted: string[];
  errors: string[];
}

export const bulkCancelBookings = (ids: string[]) =>
  request<BulkCancelResponse>('/bookings/bulk/cancel', { method: 'DELETE', body: JSON.stringify({ ids }) });

// Admin
export interface AdminResponse {
  bookings: Booking[];
  total: number;
  limit: number;
  offset: number;
  config: ConfigResponse;
  totalSlots: number;
  reservationSyncEnabled: boolean;
}

export const adminGetBookings = (params: {
  limit?: number;
  offset?: number;
  source?: string;
  resource?: string;
  search?: string;
} = {}) => {
  const q = new URLSearchParams();
  q.set('limit', String(params.limit ?? 100));
  q.set('offset', String(params.offset ?? 0));
  if (params.source) q.set('source', params.source);
  if (params.resource) q.set('resource', params.resource);
  if (params.search) q.set('search', params.search);
  return request<AdminResponse>(`/admin?${q.toString()}`);
};

export const adminDeleteBooking = (id: string) =>
  request<{ status: string }>(`/admin?id=${encodeURIComponent(id)}`, { method: 'DELETE' });

export const adminDeleteAllBookings = () =>
  request<{ status: string; count: number }>('/admin', { method: 'DELETE' });

export const adminToggleReservationSync = (enabled: boolean) =>
  request<{ reservationSyncEnabled: boolean }>('/admin/reservations', {
    method: 'POST',
    body: JSON.stringify({ enabled }),
  });

// GPU Discovery
export const adminTriggerDiscovery = () =>
  request<{ status: string; resources: GPUResource[]; totalCpu: number; totalMemory: number; flavorName: string }>('/admin/discover', { method: 'POST' });

// Database export/import
export async function adminExportDatabase(): Promise<void> {
  const resp = await fetch(`${PROXY_BASE}/admin/database/export`, {
    headers: { 'X-CSRFToken': getCSRFToken() },
  });
  if (!resp.ok) throw new Error('Export failed');
  const blob = await resp.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'bookings.db';
  a.click();
  URL.revokeObjectURL(url);
}

export async function adminImportDatabase(file: File): Promise<{ status: string }> {
  const form = new FormData();
  form.append('database', file);
  const resp = await fetch(`${PROXY_BASE}/admin/database/import`, {
    method: 'POST',
    headers: { 'X-CSRFToken': getCSRFToken() },
    body: form,
  });
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || resp.statusText);
  }
  return resp.json();
}

// Preempted workloads
export interface PreemptedWorkload {
  name: string;
  namespace: string;
  owner: string;
  reason: string;
  message: string;
  timestamp: string;
}

export interface PreemptedWorkloadsResponse {
  workloads: PreemptedWorkload[];
}

export const getPreemptedWorkloads = () => request<PreemptedWorkloadsResponse>('/workloads/preempted');

// Health
export const getHealth = () =>
  request<{ status: string; namespace: string }>('/health');

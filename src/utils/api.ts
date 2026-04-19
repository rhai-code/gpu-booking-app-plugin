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
    ...(options?.headers as Record<string, string>),
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
}

export interface BulkBookingResponse {
  bookings: Booking[];
  errors: string[];
}

export const createBulkBooking = (data: BulkBookingRequest) =>
  request<BulkBookingResponse>('/bookings/bulk', { method: 'POST', body: JSON.stringify(data) });

export const cancelBooking = (id: string) =>
  request<{ status: string }>(`/bookings?id=${encodeURIComponent(id)}`, { method: 'DELETE' });

// Admin
export interface AdminResponse {
  bookings: Booking[];
  config: ConfigResponse;
  totalSlots: number;
  reservationSyncEnabled: boolean;
}

export const adminGetBookings = () => request<AdminResponse>('/admin');

export const adminDeleteBooking = (id: string) =>
  request<{ status: string }>(`/admin?id=${encodeURIComponent(id)}`, { method: 'DELETE' });

export const adminDeleteAllBookings = () =>
  request<{ status: string; count: number }>('/admin', { method: 'DELETE' });

export const adminToggleReservationSync = (enabled: boolean) =>
  request<{ reservationSyncEnabled: boolean }>('/admin/reservations', {
    method: 'POST',
    body: JSON.stringify({ enabled }),
  });

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

// Health
export const getHealth = () =>
  request<{ status: string; namespace: string }>('/health');

// LLM Endpoints
export interface LLMEndpoint {
  id: number;
  name: string;
  url: string;
  api_key: string;
  model_name: string;
  provider_type: string;
  owner: string;
  is_global: boolean;
  enabled: boolean;
  created_at: string;
}

export interface LLMEndpointRequest {
  name: string;
  url: string;
  api_key: string;
  model_name: string;
  provider_type: string;
  is_global: boolean;
}

export const getLLMEndpoints = () =>
  request<{ endpoints: LLMEndpoint[] }>('/llm-endpoints');

export const createLLMEndpoint = (data: LLMEndpointRequest) =>
  request<LLMEndpoint>('/llm-endpoints', { method: 'POST', body: JSON.stringify(data) });

export const updateLLMEndpoint = (id: number, data: LLMEndpointRequest & { enabled: boolean }) =>
  request<{ status: string }>(`/llm-endpoints?id=${id}`, { method: 'PUT', body: JSON.stringify(data) });

export const deleteLLMEndpoint = (id: number) =>
  request<{ status: string }>(`/llm-endpoints?id=${encodeURIComponent(id)}`, { method: 'DELETE' });

export const testLLMEndpoint = (id: number) =>
  request<{ status: string; message?: string; http_status?: number }>(`/llm-endpoints/test?id=${id}`);

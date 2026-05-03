// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';

const PROXY_BASE = '/api/proxy/plugin/gpu-booking-plugin/backend/api';

// Mock fetch globally
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

// Set CSRF cookie
document.cookie = 'csrf-token=test-csrf-token';

import {
  getAuthInfo,
  getConfig,
  getBookings,
  createBooking,
  createBulkBooking,
  cancelBooking,
  bulkCancelBookings,
  adminGetBookings,
  adminDeleteBooking,
  adminDeleteAllBookings,
  adminToggleReservationSync,
  getPreemptedWorkloads,
  getHealth,
} from './api';

function jsonResponse(data: unknown, status = 200) {
  return Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    statusText: 'OK',
    json: () => Promise.resolve(data),
  });
}

function errorResponse(status: number, error: string) {
  return Promise.resolve({
    ok: false,
    status,
    statusText: error,
    json: () => Promise.resolve({ error }),
  });
}

beforeEach(() => {
  mockFetch.mockReset();
});

describe('getAuthInfo', () => {
  it('fetches /auth/me', async () => {
    const data = { username: 'alice', groups: ['devs'], is_admin: false };
    mockFetch.mockReturnValue(jsonResponse(data));

    const result = await getAuthInfo();

    expect(result).toEqual(data);
    expect(mockFetch).toHaveBeenCalledWith(
      `${PROXY_BASE}/auth/me`,
      expect.objectContaining({
        headers: expect.objectContaining({
          'X-CSRFToken': 'test-csrf-token',
        }),
      }),
    );
  });
});

describe('getConfig', () => {
  it('fetches /config', async () => {
    const data = { resources: [], bookingWindowDays: 30, totalCpu: 316, totalMemory: 3460 };
    mockFetch.mockReturnValue(jsonResponse(data));

    const result = await getConfig();
    expect(result).toEqual(data);
  });
});

describe('getBookings', () => {
  it('fetches /bookings', async () => {
    const data = { bookings: [], activeReservations: {}, currentUser: 'alice' };
    mockFetch.mockReturnValue(jsonResponse(data));

    const result = await getBookings();
    expect(result).toEqual(data);
  });
});

describe('createBooking', () => {
  it('POSTs to /bookings', async () => {
    const booking = { id: 'booking-1', user: 'alice', resource: 'nvidia.com/gpu' };
    mockFetch.mockReturnValue(jsonResponse(booking));

    const result = await createBooking({
      resource: 'nvidia.com/gpu',
      slotIndex: 0,
      date: '2025-04-25',
      slotType: 'full',
    });

    expect(result).toEqual(booking);
    const [url, opts] = mockFetch.mock.calls[0];
    expect(url).toBe(`${PROXY_BASE}/bookings`);
    expect(opts.method).toBe('POST');
    expect(JSON.parse(opts.body)).toMatchObject({ resource: 'nvidia.com/gpu' });
  });
});

describe('createBulkBooking', () => {
  it('POSTs to /bookings/bulk', async () => {
    const data = { bookings: [], errors: [] };
    mockFetch.mockReturnValue(jsonResponse(data));

    const result = await createBulkBooking({
      resources: { 'nvidia.com/gpu': 2 },
      startDate: '2025-04-25',
      endDate: '2025-04-26',
      description: 'test',
      startHour: 0,
      endHour: 24,
      utcOffset: 0,
    });

    expect(result).toEqual(data);
    const [url, opts] = mockFetch.mock.calls[0];
    expect(url).toBe(`${PROXY_BASE}/bookings/bulk`);
    expect(opts.method).toBe('POST');
  });
});

describe('cancelBooking', () => {
  it('DELETEs /bookings?id=...', async () => {
    mockFetch.mockReturnValue(jsonResponse({ status: 'deleted' }));

    const result = await cancelBooking('booking-123');
    expect(result).toEqual({ status: 'deleted' });
    expect(mockFetch.mock.calls[0][0]).toBe(`${PROXY_BASE}/bookings?id=booking-123`);
    expect(mockFetch.mock.calls[0][1].method).toBe('DELETE');
  });

  it('encodes special characters in ID', async () => {
    mockFetch.mockReturnValue(jsonResponse({ status: 'deleted' }));
    await cancelBooking('id with spaces');
    expect(mockFetch.mock.calls[0][0]).toContain('id%20with%20spaces');
  });
});

describe('bulkCancelBookings', () => {
  it('DELETEs /bookings/bulk/cancel with IDs', async () => {
    const data = { deleted: ['booking-1'], errors: [] };
    mockFetch.mockReturnValue(jsonResponse(data));

    const result = await bulkCancelBookings(['booking-1']);
    expect(result).toEqual(data);
    const [url, opts] = mockFetch.mock.calls[0];
    expect(url).toBe(`${PROXY_BASE}/bookings/bulk/cancel`);
    expect(opts.method).toBe('DELETE');
    expect(JSON.parse(opts.body)).toEqual({ ids: ['booking-1'] });
  });
});

describe('adminGetBookings', () => {
  it('fetches /admin with default params', async () => {
    const data = { bookings: [], total: 0, limit: 100, offset: 0 };
    mockFetch.mockReturnValue(jsonResponse(data));

    await adminGetBookings();
    const url = mockFetch.mock.calls[0][0] as string;
    expect(url).toContain('/admin?');
    expect(url).toContain('limit=100');
    expect(url).toContain('offset=0');
  });

  it('passes filters as query params', async () => {
    mockFetch.mockReturnValue(jsonResponse({ bookings: [], total: 0 }));

    await adminGetBookings({ source: 'consumed', resource: 'nvidia.com/gpu', search: 'alice', limit: 50, offset: 10 });
    const url = mockFetch.mock.calls[0][0] as string;
    expect(url).toContain('source=consumed');
    expect(url).toContain('resource=nvidia.com%2Fgpu');
    expect(url).toContain('search=alice');
    expect(url).toContain('limit=50');
    expect(url).toContain('offset=10');
  });
});

describe('adminDeleteBooking', () => {
  it('DELETEs /admin?id=...', async () => {
    mockFetch.mockReturnValue(jsonResponse({ status: 'deleted' }));

    await adminDeleteBooking('booking-1');
    expect(mockFetch.mock.calls[0][0]).toBe(`${PROXY_BASE}/admin?id=booking-1`);
    expect(mockFetch.mock.calls[0][1].method).toBe('DELETE');
  });
});

describe('adminDeleteAllBookings', () => {
  it('DELETEs /admin with no id', async () => {
    mockFetch.mockReturnValue(jsonResponse({ status: 'deleted', count: 5 }));

    const result = await adminDeleteAllBookings();
    expect(result.count).toBe(5);
    expect(mockFetch.mock.calls[0][0]).toBe(`${PROXY_BASE}/admin`);
  });
});

describe('adminToggleReservationSync', () => {
  it('POSTs to /admin/reservations', async () => {
    mockFetch.mockReturnValue(jsonResponse({ reservationSyncEnabled: true }));

    const result = await adminToggleReservationSync(true);
    expect(result.reservationSyncEnabled).toBe(true);
    const [url, opts] = mockFetch.mock.calls[0];
    expect(url).toBe(`${PROXY_BASE}/admin/reservations`);
    expect(JSON.parse(opts.body)).toEqual({ enabled: true });
  });
});

describe('getPreemptedWorkloads', () => {
  it('fetches /workloads/preempted', async () => {
    const data = { workloads: [] };
    mockFetch.mockReturnValue(jsonResponse(data));

    const result = await getPreemptedWorkloads();
    expect(result).toEqual(data);
  });
});

describe('getHealth', () => {
  it('fetches /health', async () => {
    const data = { status: 'ok', namespace: 'gpu-booking' };
    mockFetch.mockReturnValue(jsonResponse(data));

    const result = await getHealth();
    expect(result).toEqual(data);
  });
});

describe('error handling', () => {
  it('throws on non-ok response', async () => {
    mockFetch.mockReturnValue(errorResponse(403, 'forbidden'));

    await expect(getBookings()).rejects.toThrow('forbidden');
  });

  it('falls back to statusText when json parse fails', async () => {
    mockFetch.mockReturnValue(
      Promise.resolve({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error',
        json: () => Promise.reject(new Error('not json')),
      }),
    );

    await expect(getHealth()).rejects.toThrow('Internal Server Error');
  });
});

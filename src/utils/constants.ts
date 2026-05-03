export interface GPUResource {
  name: string;
  type: string;
  count: number;
  share: number;
  gpuEquivalent: number;
}

export interface Booking {
  id: string;
  user: string;
  email: string;
  resource: string;
  slotIndex: number;
  date: string;
  slotType: string;
  createdAt: string;
  source: string;
  description: string;
  startHour: number;
  endHour: number;
  utcOffset: number;
}

export const FALLBACK_GPU_RESOURCES: GPUResource[] = [
  { name: 'H200 Full GPU', type: 'nvidia.com/gpu', count: 8, share: 0.0625, gpuEquivalent: 1.0 },
  { name: 'MIG 3g.71gb', type: 'nvidia.com/mig-3g.71gb', count: 8, share: 0.03125, gpuEquivalent: 0.5 },
  { name: 'MIG 2g.35gb', type: 'nvidia.com/mig-2g.35gb', count: 8, share: 0.015625, gpuEquivalent: 0.25 },
  { name: 'MIG 1g.18gb', type: 'nvidia.com/mig-1g.18gb', count: 16, share: 0.0078125, gpuEquivalent: 0.125 },
];

export function buildGpuEquivalentMap(resources: GPUResource[]): Record<string, number> {
  const m: Record<string, number> = {};
  for (const r of resources) m[r.type] = r.gpuEquivalent;
  return m;
}

export function totalGpuEquivalents(resources: GPUResource[]): number {
  return resources.reduce((sum, r) => sum + r.count * r.gpuEquivalent, 0);
}

export const SLOT_TYPE = 'full';

export const RESOURCE_COLORS: Record<string, string> = {
  'nvidia.com/gpu': '#0066CC',
  'nvidia.com/mig-3g.71gb': '#5E40BE',
  'nvidia.com/mig-2g.35gb': '#009596',
  'nvidia.com/mig-1g.18gb': '#F0AB00',
};

export const FREE_COLOR = '#6A6E73';
export const CONSUMED_COLOR = '#4CB140';
export const RESERVED_COLOR = '#EE0000';

export const MONTH_NAMES = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
];

export const DAY_HEADERS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

export const DEFAULT_BOOKING_WINDOW_DAYS = 30;

export const HOUR_OPTIONS = Array.from({ length: 25 }, (_, i) => i);

export function formatDate(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  const d = String(date.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

export function todayStr(): string {
  return formatDate(new Date());
}

export function isWeekend(dateStr: string): boolean {
  const [y, m, d] = dateStr.split('-').map(Number);
  const day = new Date(y, m - 1, d).getDay();
  return day === 0 || day === 6;
}

export function getMonthDates(year: number, month: number): string[] {
  const dates: string[] = [];
  const daysInMonth = new Date(year, month + 1, 0).getDate();
  for (let d = 1; d <= daysInMonth; d++) {
    dates.push(formatDate(new Date(year, month, d)));
  }
  return dates;
}

export function getMonthStartOffset(year: number, month: number): number {
  return new Date(year, month, 1).getDay();
}

export function isInBookingWindow(dateStr: string, windowDays: number): boolean {
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const [y, m, d] = dateStr.split('-').map(Number);
  const date = new Date(y, m - 1, d);
  const maxDate = new Date(today);
  maxDate.setDate(maxDate.getDate() + windowDays);
  return date >= today && date < maxDate;
}

export function isPastDate(dateStr: string): boolean {
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const [y, m, d] = dateStr.split('-').map(Number);
  const date = new Date(y, m - 1, d);
  return date < today;
}

export function getUtcOffsetHours(dateStr: string): number {
  const [y, m, d] = dateStr.split('-').map(Number);
  return -new Date(y, m - 1, d).getTimezoneOffset() / 60;
}

export function formatHour(h: number): string {
  if (h === 24) return '00:00 +1d';
  return `${String(h).padStart(2, '0')}:00`;
}

export function getDateRange(a: string, b: string): string[] {
  const [startStr, endStr] = a < b ? [a, b] : [b, a];
  const [sy, sm, sd] = startStr.split('-').map(Number);
  const [ey, em, ed] = endStr.split('-').map(Number);
  const start = new Date(sy, sm - 1, sd);
  const end = new Date(ey, em - 1, ed);
  const dates: string[] = [];
  for (const cur = new Date(start); cur <= end; cur.setDate(cur.getDate() + 1)) {
    dates.push(formatDate(cur));
  }
  return dates;
}

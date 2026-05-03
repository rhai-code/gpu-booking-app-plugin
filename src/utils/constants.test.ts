import { describe, it, expect } from 'vitest';
import {
  formatDate,
  todayStr,
  isWeekend,
  getMonthDates,
  getMonthStartOffset,
  getDateRange,
  formatHour,
  getUtcOffsetHours,
  isPastDate,
  buildGpuEquivalentMap,
  totalGpuEquivalents,
  FALLBACK_GPU_RESOURCES,
} from './constants';

describe('formatDate', () => {
  it('formats date as YYYY-MM-DD', () => {
    expect(formatDate(new Date(2025, 0, 5))).toBe('2025-01-05');
  });

  it('pads single-digit month and day', () => {
    expect(formatDate(new Date(2025, 2, 3))).toBe('2025-03-03');
  });

  it('handles December 31', () => {
    expect(formatDate(new Date(2025, 11, 31))).toBe('2025-12-31');
  });
});

describe('todayStr', () => {
  it('returns a YYYY-MM-DD string', () => {
    expect(todayStr()).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });
});

describe('isWeekend', () => {
  it('returns true for Saturday', () => {
    // 2025-04-26 is Saturday
    expect(isWeekend('2025-04-26')).toBe(true);
  });

  it('returns true for Sunday', () => {
    // 2025-04-27 is Sunday
    expect(isWeekend('2025-04-27')).toBe(true);
  });

  it('returns false for weekday', () => {
    // 2025-04-24 is Thursday
    expect(isWeekend('2025-04-24')).toBe(false);
  });
});

describe('getMonthDates', () => {
  it('returns 30 dates for April', () => {
    const dates = getMonthDates(2025, 3); // month is 0-indexed
    expect(dates).toHaveLength(30);
    expect(dates[0]).toBe('2025-04-01');
    expect(dates[29]).toBe('2025-04-30');
  });

  it('returns 28 dates for Feb non-leap year', () => {
    const dates = getMonthDates(2025, 1);
    expect(dates).toHaveLength(28);
  });

  it('returns 29 dates for Feb leap year', () => {
    const dates = getMonthDates(2024, 1);
    expect(dates).toHaveLength(29);
  });

  it('returns 31 dates for January', () => {
    const dates = getMonthDates(2025, 0);
    expect(dates).toHaveLength(31);
  });
});

describe('getMonthStartOffset', () => {
  it('returns day-of-week for first of month', () => {
    // April 2025 starts on Tuesday (2)
    expect(getMonthStartOffset(2025, 3)).toBe(2);
  });

  it('returns 0 for month starting on Sunday', () => {
    // June 2025 starts on Sunday
    expect(getMonthStartOffset(2025, 5)).toBe(0);
  });
});

describe('getDateRange', () => {
  it('returns inclusive range between two dates', () => {
    const range = getDateRange('2025-04-01', '2025-04-03');
    expect(range).toEqual(['2025-04-01', '2025-04-02', '2025-04-03']);
  });

  it('handles reversed order', () => {
    const range = getDateRange('2025-04-03', '2025-04-01');
    expect(range).toEqual(['2025-04-01', '2025-04-02', '2025-04-03']);
  });

  it('returns single date when start equals end', () => {
    const range = getDateRange('2025-04-15', '2025-04-15');
    expect(range).toEqual(['2025-04-15']);
  });

  it('spans month boundaries', () => {
    const range = getDateRange('2025-03-30', '2025-04-02');
    expect(range).toEqual(['2025-03-30', '2025-03-31', '2025-04-01', '2025-04-02']);
  });
});

describe('formatHour', () => {
  it('formats 0 as 00:00', () => {
    expect(formatHour(0)).toBe('00:00');
  });

  it('formats 9 as 09:00', () => {
    expect(formatHour(9)).toBe('09:00');
  });

  it('formats 14 as 14:00', () => {
    expect(formatHour(14)).toBe('14:00');
  });

  it('formats 24 as 00:00 +1d', () => {
    expect(formatHour(24)).toBe('00:00 +1d');
  });
});

describe('buildGpuEquivalentMap', () => {
  it('maps resource type to GPU equivalent', () => {
    const map = buildGpuEquivalentMap(FALLBACK_GPU_RESOURCES);
    expect(map['nvidia.com/gpu']).toBe(1.0);
    expect(map['nvidia.com/mig-3g.71gb']).toBe(0.5);
    expect(map['nvidia.com/mig-2g.35gb']).toBe(0.25);
    expect(map['nvidia.com/mig-1g.18gb']).toBe(0.125);
  });

  it('returns empty map for empty input', () => {
    const map = buildGpuEquivalentMap([]);
    expect(Object.keys(map)).toHaveLength(0);
  });
});

describe('totalGpuEquivalents', () => {
  it('sums count * gpuEquivalent for all resources', () => {
    // 8*1.0 + 8*0.5 + 8*0.25 + 16*0.125 = 8 + 4 + 2 + 2 = 16
    expect(totalGpuEquivalents(FALLBACK_GPU_RESOURCES)).toBe(16);
  });

  it('returns 0 for empty input', () => {
    expect(totalGpuEquivalents([])).toBe(0);
  });

  it('calculates for single resource', () => {
    expect(totalGpuEquivalents([{ name: 'X', type: 'x', count: 4, share: 0.1, gpuEquivalent: 0.5 }])).toBe(2);
  });
});

describe('getUtcOffsetHours', () => {
  it('returns a number for a valid date', () => {
    const offset = getUtcOffsetHours('2025-06-15');
    expect(typeof offset).toBe('number');
  });

  it('returns consistent offset for the same date', () => {
    const a = getUtcOffsetHours('2025-01-15');
    const b = getUtcOffsetHours('2025-01-15');
    expect(a).toBe(b);
  });

  it('offset is within valid range (-12 to +14)', () => {
    const offset = getUtcOffsetHours('2025-06-15');
    expect(offset).toBeGreaterThanOrEqual(-12);
    expect(offset).toBeLessThanOrEqual(14);
  });

  it('is the inverse of getTimezoneOffset', () => {
    const date = new Date(2025, 5, 15);
    const expected = -date.getTimezoneOffset() / 60;
    expect(getUtcOffsetHours('2025-06-15')).toBe(expected);
  });
});

describe('isPastDate', () => {
  it('returns true for a date well in the past', () => {
    expect(isPastDate('2020-01-01')).toBe(true);
  });

  it('returns false for a date well in the future', () => {
    expect(isPastDate('2099-12-31')).toBe(false);
  });

  it('returns false for today', () => {
    expect(isPastDate(todayStr())).toBe(false);
  });
});

import { describe, expect, it } from 'vitest';
import { acceptanceScenarioButtons, isValidTimeShiftHours, latestGraceEntryAt, maxTimeShiftHours, retrySchedule, timeShiftPresets } from './time-travel';

// Значения пресетов — реестр TimeShiftPreset
// apps/backend/internal/billing/application/admin_time_travel_service.go.
const backendTimeShiftPresets = ['period_expired', 'retry_24h_due', 'retry_72h_due', 'reminder_window'] as const;

describe('timeShiftPresets', () => {
  it('covers every backend TimeShiftPreset value exactly once', () => {
    for (const preset of backendTimeShiftPresets) {
      expect(timeShiftPresets.filter((id) => id === preset)).toHaveLength(1);
    }
    expect(timeShiftPresets).toHaveLength(backendTimeShiftPresets.length);
  });
});

describe('acceptanceScenarioButtons', () => {
  it('offers the five acceptance scenarios of the full lifecycle walk (#648)', () => {
    expect(acceptanceScenarioButtons.map((button) => button.name)).toEqual([
      'Войти в grace',
      'В окно ретрая +24',
      'В окно ретрая +72',
      'В окно напоминания',
      'Истечь из grace',
    ]);
  });

  it('maps every button onto a backend preset, covering all four', () => {
    for (const button of acceptanceScenarioButtons) {
      expect(timeShiftPresets).toContain(button.preset);
    }
    for (const preset of timeShiftPresets) {
      expect(acceptanceScenarioButtons.some((button) => button.preset === preset)).toBe(true);
    }
  });
});

// Лимит одного сдвига ±90 дней (MaxTimeShift) —
// apps/backend/internal/billing/application/config.go.
describe('isValidTimeShiftHours', () => {
  it('accepts a signed integer shift within ±90 days', () => {
    expect(maxTimeShiftHours).toBe(2160);
    expect(isValidTimeShiftHours(1)).toBe(true);
    expect(isValidTimeShiftHours(-25)).toBe(true);
    expect(isValidTimeShiftHours(2160)).toBe(true);
    expect(isValidTimeShiftHours(-2160)).toBe(true);
  });

  it('rejects zero, out-of-range and fractional shifts', () => {
    expect(isValidTimeShiftHours(0)).toBe(false);
    expect(isValidTimeShiftHours(2161)).toBe(false);
    expect(isValidTimeShiftHours(-2161)).toBe(false);
    expect(isValidTimeShiftHours(1.5)).toBe(false);
    expect(isValidTimeShiftHours(Number.NaN)).toBe(false);
  });
});

// Анкер ретраев долга — created_at последнего перехода grace_entered
// (фаза ретраев читает именно его, phases.go).
describe('latestGraceEntryAt', () => {
  it('returns null without transitions or without a grace entry', () => {
    expect(latestGraceEntryAt([])).toBeNull();
    expect(latestGraceEntryAt([{ createdAt: '2026-09-14T10:00:00Z', reason: 'payment_applied' }])).toBeNull();
  });

  it('picks the latest grace_entered among the transitions', () => {
    const anchor = latestGraceEntryAt([
      { createdAt: '2026-09-14T10:00:00Z', reason: 'grace_entered' },
      { createdAt: '2026-09-12T08:00:00Z', reason: 'grace_entered' },
      { createdAt: '2026-09-14T12:00:00Z', reason: 'time_shifted' },
    ]);
    expect(anchor).toBe(Date.parse('2026-09-14T10:00:00Z'));
  });

  it('ignores entries without a parseable timestamp', () => {
    expect(latestGraceEntryAt([{ createdAt: null, reason: 'grace_entered' }])).toBeNull();
    expect(latestGraceEntryAt([{ createdAt: 'not-a-date', reason: 'grace_entered' }])).toBeNull();
  });
});

// Расписание ретраев долга — +24 ч и +72 ч от анкера
// (graceRetryFirstAfter/SecondAfter, phases.go).
describe('retrySchedule', () => {
  const anchor = Date.parse('2026-09-14T10:00:00Z');

  it('returns null without an anchor', () => {
    expect(retrySchedule(null, new Date())).toBeNull();
  });

  it('places the two retries at +24 h and +72 h with due flags against now', () => {
    const now = new Date(Date.parse('2026-09-15T11:00:00Z'));
    const schedule = retrySchedule(anchor, now);
    expect(schedule).toEqual([
      { label: 'Ретрай +24 ч', at: new Date(Date.parse('2026-09-15T10:00:00Z')), due: true },
      { label: 'Ретрай +72 ч', at: new Date(Date.parse('2026-09-17T10:00:00Z')), due: false },
    ]);
  });

  it('marks a future schedule entirely not due', () => {
    const now = new Date(Date.parse('2026-09-14T11:00:00Z'));
    expect(retrySchedule(anchor, now)?.every((point) => !point.due)).toBe(true);
  });
});

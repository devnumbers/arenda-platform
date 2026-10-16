import { describe, expect, it } from 'vitest';
import {
  canCancelOnBehalf,
  canExtendGrace,
  paidRemainderWarning,
  remainderDays,
  transitionInitiatorChoices,
  transitionReasonLabel,
} from './subscription';

// Реестр TransitionReason из apps/backend/internal/billing/domain/transition.go.
const domainTransitionReasons = [
  'registered',
  'cancelled',
  'downgrade_scheduled',
  'payment_applied',
  'grace_entered',
  'grace_extended',
  'scheduled_change_applied',
  'expired',
  'refunded',
  'service_assigned',
  'forced_change',
] as const;

describe('transitionReasonLabel', () => {
  it('labels every domain TransitionReason value in Russian', () => {
    for (const reason of domainTransitionReasons) {
      const label = transitionReasonLabel(reason);
      expect(label.trim()).not.toBe('');
      expect(label).not.toBe(reason);
    }
  });

  it('passes unknown reasons through unchanged (the log is append-only)', () => {
    expect(transitionReasonLabel('future_reason')).toBe('future_reason');
  });
});

describe('transitionInitiatorChoices', () => {
  it('covers user/admin/system with non-empty labels', () => {
    expect(transitionInitiatorChoices.map((choice) => choice.id)).toEqual(['user', 'admin', 'system']);
    for (const choice of transitionInitiatorChoices) {
      expect(choice.name.trim()).not.toBe('');
    }
  });
});

describe('remainderDays', () => {
  const now = new Date('2026-08-15T10:00:00Z');

  it('returns null without a validity date', () => {
    expect(remainderDays(null, now)).toBeNull();
    expect(remainderDays(undefined, now)).toBeNull();
  });

  it('returns null for an unparsable date', () => {
    expect(remainderDays('not-a-date', now)).toBeNull();
  });

  it('rounds a partial day up', () => {
    expect(remainderDays('2026-08-16T09:59:59Z', now)).toBe(1);
    expect(remainderDays('2026-08-16T10:00:01Z', now)).toBe(2);
  });

  it('clamps an expired date to zero', () => {
    expect(remainderDays('2026-08-14T00:00:00Z', now)).toBe(0);
  });
});

describe('paidRemainderWarning', () => {
  const now = new Date('2026-08-15T10:00:00Z');

  it('warns about the paid remainder of a paid subscription', () => {
    const warning = paidRemainderWarning(
      { source: 'paid', status: 'active', validUntil: '2026-08-18T10:00:00Z' },
      now
    );
    expect(warning).toContain('3 дней');
    expect(warning).toContain('не возвращается');
  });

  it('uses the singular day form for exactly one day', () => {
    expect(paidRemainderWarning({ source: 'paid', status: 'active', validUntil: '2026-08-16T10:00:00Z' }, now)).toContain('1 день');
  });

  it('stays silent for a service subscription', () => {
    expect(paidRemainderWarning({ source: 'service', status: 'active', validUntil: '2026-08-18T10:00:00Z' }, now)).toBeNull();
  });

  it('stays silent when the paid period has run out', () => {
    expect(paidRemainderWarning({ source: 'paid', status: 'active', validUntil: '2026-08-14T10:00:00Z' }, now)).toBeNull();
  });

  it('stays silent without a validity date', () => {
    expect(paidRemainderWarning({ source: 'paid', status: 'active', validUntil: null }, now)).toBeNull();
  });
});

describe('operation availability', () => {
  it('allows cancelling on behalf only a paid subscription that is not cancelled', () => {
    expect(canCancelOnBehalf({ source: 'paid', status: 'active' })).toBe(true);
    expect(canCancelOnBehalf({ source: 'paid', status: 'grace' })).toBe(true);
    expect(canCancelOnBehalf({ source: 'paid', status: 'cancelled' })).toBe(false);
    expect(canCancelOnBehalf({ source: 'service', status: 'active' })).toBe(false);
  });

  it('allows extending grace only in grace', () => {
    expect(canExtendGrace({ source: 'paid', status: 'grace' })).toBe(true);
    expect(canExtendGrace({ source: 'paid', status: 'active' })).toBe(false);
    expect(canExtendGrace({ source: 'paid', status: 'cancelled' })).toBe(false);
  });
});

import { describe, expect, it } from 'vitest';
import type {
  FreeReminderCreateRequest,
  FreeReminderUpdateRequest,
} from '../model/types';
import { toCreateWireRequest, toUpdateWireRequest } from './hooks';

describe('free-reminders wire serializers', () => {
  it('maps triggerAt to trigger_at on create', () => {
    const command: FreeReminderCreateRequest = {
      title: 'Оплатить интернет',
      triggerAt: '2026-09-01T10:00:00Z',
      periodicity: 'monthly',
    };

    expect(toCreateWireRequest(command)).toStrictEqual({
      title: 'Оплатить интернет',
      trigger_at: '2026-09-01T10:00:00Z',
      periodicity: 'monthly',
    });
  });

  it('maps triggerAt to trigger_at on update', () => {
    const command: FreeReminderUpdateRequest = {
      title: 'Оплатить интернет и воду',
      triggerAt: '2026-09-02T09:00:00Z',
      periodicity: undefined,
    };

    expect(toUpdateWireRequest(command)).toStrictEqual({
      title: 'Оплатить интернет и воду',
      trigger_at: '2026-09-02T09:00:00Z',
      periodicity: undefined,
    });
  });
});

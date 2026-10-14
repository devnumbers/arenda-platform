import { describe, expect, it } from 'vitest';
import {
  consumeRentalCompleteResume,
  fulfillRentalCompleteResume,
  requestRentalCompleteResume,
} from './rental-complete-resume-session';

describe('rental-complete-resume-session (продолжение статуса после завершения, #628)', () => {
  it('без запроса consume возвращает null', () => {
    expect(consumeRentalCompleteResume('p1')).toBeNull();
  });

  it('запрос без завершения (мастер закрыли) статус не даёт — и метка гасится', () => {
    requestRentalCompleteResume('p1', 'archive');
    expect(consumeRentalCompleteResume('p1')).toBeNull();
    expect(consumeRentalCompleteResume('p1')).toBeNull();
  });

  it('запрос → успешное завершение в мастере → consume отдаёт действие один раз', () => {
    requestRentalCompleteResume('p1', 'archive');
    fulfillRentalCompleteResume('p1');
    expect(consumeRentalCompleteResume('p1')).toBe('archive');
    expect(consumeRentalCompleteResume('p1')).toBeNull();
  });

  it('метка «ремонт» доезжает так же', () => {
    requestRentalCompleteResume('p1', 'maintenance');
    fulfillRentalCompleteResume('p1');
    expect(consumeRentalCompleteResume('p1')).toBe('maintenance');
  });

  it('повторный запрос перезаписывает действие — гард сработал заново с другим статусом', () => {
    requestRentalCompleteResume('p1', 'archive');
    requestRentalCompleteResume('p1', 'maintenance');
    fulfillRentalCompleteResume('p1');
    expect(consumeRentalCompleteResume('p1')).toBe('maintenance');
  });

  it('ключ — объект: чужая страница не потребляет и не гасит чужую метку', () => {
    requestRentalCompleteResume('p1', 'archive');
    expect(consumeRentalCompleteResume('p2')).toBeNull();
    fulfillRentalCompleteResume('p1');
    expect(consumeRentalCompleteResume('p2')).toBeNull();
    expect(consumeRentalCompleteResume('p1')).toBe('archive');
  });

  it('fulfil без запроса — no-op', () => {
    fulfillRentalCompleteResume('p1');
    expect(consumeRentalCompleteResume('p1')).toBeNull();
  });

  it('новый запрос после исполненной и потреблённой метки работает заново', () => {
    requestRentalCompleteResume('p1', 'archive');
    fulfillRentalCompleteResume('p1');
    consumeRentalCompleteResume('p1');
    requestRentalCompleteResume('p1', 'maintenance');
    fulfillRentalCompleteResume('p1');
    expect(consumeRentalCompleteResume('p1')).toBe('maintenance');
    expect(consumeRentalCompleteResume('p1')).toBeNull();
  });
});

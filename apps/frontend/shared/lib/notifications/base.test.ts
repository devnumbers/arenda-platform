import { beforeEach, describe, expect, it, vi } from 'vitest';

const toast = vi.hoisted(() => ({
  dismiss: vi.fn(),
  success: vi.fn(() => 't-success'),
  error: vi.fn(() => 't-error'),
  info: vi.fn(() => 't-info'),
  warning: vi.fn(() => 't-warning'),
  loading: vi.fn(() => 't-loading'),
  promise: vi.fn(),
}));

vi.mock('react-toastify/unstyled', () => ({ toast }));

vi.mock('@/shared/ui/toast/ToastProvider', () => ({
  TOAST_AUTO_CLOSE_MS: 5000,
  CloseToastButton: () => null,
}));

import { notify } from './base';

/** Порядок вызовов dismiss → показ внутри каждого метода notify. */
function showOrder(): string[] {
  const order: string[] = [];
  toast.dismiss.mockImplementation(() => order.push('dismiss'));
  for (const show of [toast.success, toast.error, toast.info, toast.warning, toast.loading]) {
    show.mockImplementation(() => {
      order.push('show');
      return `t-${order.length}`;
    });
  }
  return order;
}

describe('notify — вытесняющий показ (решение владельца 13.10: максимум один тост на экране)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('каждый показ вытесняет активный тост: dismiss строго до показа, новый въезжает одновременно', () => {
    const order = showOrder();

    for (const show of [
      () => notify.success('A'),
      () => notify.error('B'),
      () => notify.error(new Error('C')),
      () => notify.info('D'),
      () => notify.warning('E'),
      () => notify.loading('F'),
    ]) {
      order.length = 0;
      show();
      expect(order).toEqual(['dismiss', 'show']);
    }
  });

  it('promise вытесняет активный тост перед запуском pending-тоста', () => {
    const order = showOrder();
    toast.promise.mockImplementation(() => order.push('promise'));

    void notify.promise(Promise.resolve(1), {
      loading: 'Загрузка',
      success: 'Готово',
      error: 'Ошибка',
    });

    expect(order).toEqual(['dismiss', 'promise']);
  });

  it('close гасит адресно по ключу — это не вытесняющий показ', () => {
    notify.close('t-9');

    expect(toast.dismiss).toHaveBeenCalledTimes(1);
    expect(toast.dismiss).toHaveBeenCalledWith('t-9');
  });
});

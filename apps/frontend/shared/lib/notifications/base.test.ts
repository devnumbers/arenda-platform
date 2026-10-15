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

const outcomeHaptic = vi.hoisted(() => vi.fn());

vi.mock('react-toastify/unstyled', () => ({ toast }));

vi.mock('@/shared/ui/design/action-haptics', () => ({ outcomeHaptic }));

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

describe('notify — хаптика исходов (план 018: вибрация вместе с видимым сигналом)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  /** Прогон микрофлотча: промисы promise-пути сеттлятся в микроочереди. */
  async function flushMicrotasks(): Promise<void> {
    await new Promise<void>((resolve) => setTimeout(resolve, 0));
  }

  it('success и error (обе перегрузки) зовут хаптику исхода', () => {
    notify.success('A');
    notify.error('B');
    notify.error(new Error('C'));

    expect(outcomeHaptic).toHaveBeenCalledWith('success');
    expect(outcomeHaptic).toHaveBeenCalledWith('error');
    expect(outcomeHaptic).toHaveBeenCalledTimes(3);
  });

  it('info/warning/loading молчат — utility §13', () => {
    notify.info('D');
    notify.warning('E');
    notify.loading('F');

    expect(outcomeHaptic).not.toHaveBeenCalled();
  });

  it('promise: resolved зовёт success-хаптику', async () => {
    notify.promise(Promise.resolve(1), {
      loading: 'Загрузка',
      success: 'Готово',
      error: 'Ошибка',
    });
    await flushMicrotasks();

    expect(outcomeHaptic).toHaveBeenCalledWith('success');
  });

  it('promise: rejected зовёт error-хаптику', async () => {
    notify.promise(Promise.reject(new Error('x')), {
      loading: 'Загрузка',
      success: 'Готово',
      error: 'Ошибка',
    });
    await flushMicrotasks();

    expect(outcomeHaptic).toHaveBeenCalledWith('error');
  });
});

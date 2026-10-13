import { beforeEach, describe, expect, it, vi } from 'vitest';

const toast = vi.hoisted(() => ({
  dismiss: vi.fn(),
}));

vi.mock('react-toastify/unstyled', () => ({ toast }));

import { showDisplacingToast } from './show-displacing-toast';

describe('showDisplacingToast — максимум один тост на экране (решение владельца 13.10)', () => {
  beforeEach(() => {
    toast.dismiss.mockClear();
  });

  it('гасит активные тосты ДО показа нового — старый играет exit, новый въезжает одновременно (кросс-свап)', () => {
    const order: string[] = [];
    toast.dismiss.mockImplementation(() => order.push('dismiss'));
    const show = vi.fn(() => {
      order.push('show');
      return 't-1';
    });

    const result = showDisplacingToast(show);

    expect(result).toBe('t-1');
    expect(show).toHaveBeenCalledTimes(1);
    expect(order).toEqual(['dismiss', 'show']);
  });

  it('dismiss() без аргументов — гасятся все активные тосты, «догоняющих» не остаётся', () => {
    showDisplacingToast(() => 't-2');

    expect(toast.dismiss).toHaveBeenCalledWith();
    expect(toast.dismiss).toHaveBeenCalledTimes(1);
  });
});

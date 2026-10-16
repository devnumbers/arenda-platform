import { describe, expect, it } from 'vitest';

import type { ApiError } from '@/shared/api/errors';
import { inviteServerError, INVITE_EMAIL_ERROR, INVITE_GENERIC_ERROR } from './use-invite-form';

describe('inviteServerError', () => {
  it('семантический 400 с detail — текст бэка', () => {
    expect(
      inviteServerError({ status: 400, detail: 'Нельзя пригласить самого себя' } as ApiError),
    ).toBe('Нельзя пригласить самого себя');
  });

  it('400 без detail и инфраструктура — общая фраза', () => {
    expect(inviteServerError({ status: 400, detail: '' } as ApiError)).toBe(INVITE_GENERIC_ERROR);
    expect(inviteServerError({ status: 500, detail: 'x' } as ApiError)).toBe(INVITE_GENERIC_ERROR);
  });

  it('подпись валидации почты — канон формы', () => {
    expect(INVITE_EMAIL_ERROR).toBe('Укажите корректную электронную почту');
  });
});

import { describe, expect, it } from 'vitest';
import { validateLoginDraft } from './use-login-draft';

/** Черновик входа — чистый валидатор (канон use-payment-wizard-draft.test.ts):
 * хук сам не тестируется (node-env без рендера), шов — нормализация payload'а
 * из хранилища. */
describe('validateLoginDraft', () => {
  const empty = { step: 'email', email: '' } as const;

  it('невалидный payload → пустой черновик на шаге почты', () => {
    expect(validateLoginDraft(null)).toStrictEqual(empty);
    expect(validateLoginDraft('строка')).toStrictEqual(empty);
    expect(validateLoginDraft(42)).toStrictEqual(empty);
    expect(validateLoginDraft({ step: 'мусор' })).toStrictEqual(empty);
    expect(validateLoginDraft({ step: 'email', email: 42 })).toStrictEqual(empty);
  });

  it('телефонный черновик прежней версии роняется в дефолт (телефон покинул вход, ADR 0067)', () => {
    expect(
      validateLoginDraft({ step: 'phone', phone: '+7 (926) 111-22-33', email: '' }),
    ).toStrictEqual(empty);
  });

  it('шаг почты хранит набранный адрес, чужие поля хранилища отбрасываются', () => {
    expect(
      validateLoginDraft({ step: 'email', email: 'new-user@example.com', phone: '+79261112233' }),
    ).toStrictEqual({ step: 'email', email: 'new-user@example.com' });
  });

  it('шаг кода живёт только с валидной почтой — иначе рестарт на шаг почты', () => {
    expect(validateLoginDraft({ step: 'code', email: 'new-user@example.com' })).toStrictEqual({
      step: 'code',
      email: 'new-user@example.com',
    });
    // Шаг сбрасывается, набранное — нет (канон прежнего валидатора: поле
    // не стирается, стирать набранное — сюрприз).
    expect(validateLoginDraft({ step: 'code', email: 'не-почта' })).toStrictEqual({
      step: 'email',
      email: 'не-почта',
    });
  });
});

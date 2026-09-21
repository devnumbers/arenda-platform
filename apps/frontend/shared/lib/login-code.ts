/**
 * Логин-код (identity: цели login / phone_change / email_change) — 6 цифр.
 * Маска и валидность шага ввода кода: нецифровые знаки отбрасываются,
 * «Продолжить» флоу включается ровно шестью цифрами.
 */
export const LOGIN_CODE_LENGTH = 6;

const fullCode = new RegExp(`^\\d{${LOGIN_CODE_LENGTH}}$`);

/** Маска ввода кода: только цифры, не длиннее кода. */
export function loginCodeFromInput(raw: string): string {
  return raw.replace(/\D/g, '').slice(0, LOGIN_CODE_LENGTH);
}

/** Код введён полностью — валидность формы шага кода. */
export function isValidLoginCode(value: string): boolean {
  return fullCode.test(value);
}

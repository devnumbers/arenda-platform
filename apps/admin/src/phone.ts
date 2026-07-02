const RUSSIAN_MOBILE_PHONE_REGEX = /^\+79\d{9}$/;

export function formatPhoneInput(input: string): string {
  let digits = input.replace(/\D/g, '');
  if (digits.startsWith('7') || digits.startsWith('8')) {
    digits = digits.slice(1);
  }
  digits = digits.slice(0, 10);

  let formatted = '+7';
  const operator = digits.slice(0, 3);
  const first = digits.slice(3, 6);
  const second = digits.slice(6, 8);
  const third = digits.slice(8, 10);

  if (operator) {
    formatted += ` (${operator}`;
  }
  if (first) {
    formatted += `) ${first}`;
  }
  if (second) {
    formatted += `-${second}`;
  }
  if (third) {
    formatted += `-${third}`;
  }

  return formatted;
}

export function normalizePhone(input: string): string {
  let digits = input.replace(/\D/g, '');
  if (digits.startsWith('8')) {
    digits = `7${digits.slice(1)}`;
  }
  if (!digits || digits === '7') {
    return '';
  }
  if (!digits.startsWith('7')) {
    digits = `7${digits}`;
  }
  return `+${digits}`;
}

export function isValidPhone(input: string): boolean {
  return RUSSIAN_MOBILE_PHONE_REGEX.test(normalizePhone(input));
}

export function formatPhoneInput(input: string): string {
  let cleaned = input.replace(/\D/g, '');
  if (cleaned.startsWith('7')) cleaned = cleaned.slice(1);
  if (cleaned.startsWith('8')) cleaned = cleaned.slice(1);
  cleaned = cleaned.slice(0, 10);
  const match = cleaned.match(/^(\d{0,3})(\d{0,3})(\d{0,2})(\d{0,2})$/);
  let formatted = '+7';
  if (match) {
    if (match[1]) formatted += ` (${match[1]}`;
    if (match[2]) formatted += `) ${match[2]}`;
    if (match[3]) formatted += `-${match[3]}`;
    if (match[4]) formatted += `-${match[4]}`;
  }
  return formatted.slice(0, 18);
}

export function normalizePhone(formatted: string): string {
  let digits = formatted.replace(/\D/g, '');
  if (digits.startsWith('8')) digits = digits.slice(1);
  if (digits === '' || digits === '7') return '';
  return digits.startsWith('7') ? `+${digits}` : `+7${digits}`;
}

export function isPhoneValid(formatted: string): boolean {
  const normalized = normalizePhone(formatted);
  return /^\+7\d{10}$/.test(normalized);
}

/** Канонический +7XXXXXXXXXX → маска +7 (XXX) XXX-XX-XX для отображения
 * (деталка контакта #510). Не-каноническое значение возвращается как есть. */
export function formatPhoneDisplay(canonical: string): string {
  const digits = canonical.replace(/\D/g, '');
  if (digits.length !== 11 || !digits.startsWith('7')) return canonical;
  return `+7 (${digits.slice(1, 4)}) ${digits.slice(4, 7)}-${digits.slice(7, 9)}-${digits.slice(9, 11)}`;
}

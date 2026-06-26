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
  const digits = formatted.replace(/\D/g, '');
  if (digits === '' || digits === '7') return '';
  return digits.startsWith('7') ? `+${digits}` : `+7${digits}`;
}

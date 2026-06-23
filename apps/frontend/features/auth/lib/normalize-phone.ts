export function normalizePhone(formatted: string): string {
  const digits = formatted.replace(/\D/g, '');
  return digits.startsWith('7') ? `+${digits}` : `+7${digits}`;
}

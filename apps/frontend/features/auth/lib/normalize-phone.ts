export function normalizePhone(formatted: string): string {
  const digits = formatted.replace(/\D/g, '');
  return `+7${digits}`;
}

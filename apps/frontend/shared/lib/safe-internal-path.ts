/** Allow only internal absolute paths; reject protocol-relative URLs (`//` and `/\` — WHATWG URL normalizes `\` to `/`) and /login redirect loops. */
export function safeInternalPath(value: string | null | undefined): string | null {
  if (
    !value ||
    !value.startsWith('/') ||
    value.startsWith('//') ||
    value.startsWith('/\\') ||
    value.startsWith('/login')
  ) {
    return null;
  }
  return value;
}

import * as fs from 'fs';

export const BACKEND_LOG = '/Users/smirnowwwivan/Nambers/arenda-planform/backend_runtime.log';
export const FRONTEND_LOG = '/Users/smirnowwwivan/Nambers/arenda-planform/frontend_runtime.log';

export function getLogOffset(path: string): number {
  try {
    return fs.statSync(path).size;
  } catch {
    return 0;
  }
}

export function getLogTail(path: string, offset: number): string {
  try {
    const data = fs.readFileSync(path, 'utf8');
    return data.slice(offset);
  } catch {
    return '';
  }
}

export function assertNoBackendErrors(offset: number): void {
  const tail = getLogTail(BACKEND_LOG, offset);
  const errorLines = tail
    .split('\n')
    .filter((line) => line.match(/\berror\b|\bfatal\b|\bpanic\b/i));
  if (errorLines.length > 0) {
    throw new Error(`Backend errors detected during test:\n${errorLines.join('\n')}`);
  }
}

export function assertNoFrontendErrors(offset: number): void {
  const tail = getLogTail(FRONTEND_LOG, offset);
  const errorLines = tail
    .split('\n')
    .filter(
      (line) =>
        line.includes('error') || line.includes('Error') || line.includes('uncaughtException') || line.includes('unhandledRejection')
    );
  if (errorLines.length > 0) {
    throw new Error(`Frontend errors detected during test:\n${errorLines.join('\n')}`);
  }
}

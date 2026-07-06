export class ApiError extends Error {
  constructor(
    public code: string,
    public detail: string,
    public requestId?: string,
    public status?: number,
    public cause?: unknown,
    public retryAfter?: number,
  ) {
    super(detail, { cause });
    this.name = 'ApiError';
  }
}

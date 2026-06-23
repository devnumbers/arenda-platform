export class ApiError extends Error {
  constructor(
    public code: string,
    public detail: string,
    public requestId?: string,
    public status?: number,
  ) {
    super(detail);
    this.name = 'ApiError';
  }
}

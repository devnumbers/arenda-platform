export type FieldError = {
    readonly field: string;
    readonly detail: string;
};

export class ApiError extends Error {
    constructor(
        public code: string,
        public detail: string,
        public requestId?: string,
        public status?: number,
        public cause?: unknown,
        public retryAfter?: number,
        public fieldErrors?: readonly FieldError[],
    ) {
        super(detail, {cause});
        this.name = 'ApiError';
    }
}

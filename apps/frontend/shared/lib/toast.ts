import { toast } from '@heroui/react/toast';
import { ApiError } from '@/shared/api/errors';

export type ToastOptions = {
  description?: string;
  duration?: number;
};

const DEFAULT_DURATIONS = {
  success: 4000,
  error: 6000,
  info: 5000,
  warning: 6000,
};

function normalizeError(error: unknown): string {
  if (error instanceof ApiError) {
    return error.detail || 'Произошла ошибка';
  }
  if (error instanceof Error) {
    return error.message;
  }
  if (typeof error === 'string') {
    return error;
  }
  return 'Произошла ошибка';
}

export const notify = {
  success: (title: string, options?: ToastOptions) =>
    toast.success(title, {
      description: options?.description,
      timeout: options?.duration ?? DEFAULT_DURATIONS.success,
    }),

  error: (titleOrError: string | unknown, options?: ToastOptions) => {
    const title =
      typeof titleOrError === 'string'
        ? titleOrError
        : normalizeError(titleOrError);
    return toast.danger(title, {
      description: options?.description,
      timeout: options?.duration ?? DEFAULT_DURATIONS.error,
    });
  },

  info: (title: string, options?: ToastOptions) =>
    toast.info(title, {
      description: options?.description,
      timeout: options?.duration ?? DEFAULT_DURATIONS.info,
    }),

  warning: (title: string, options?: ToastOptions) =>
    toast.warning(title, {
      description: options?.description,
      timeout: options?.duration ?? DEFAULT_DURATIONS.warning,
    }),

  promise: <T>(
    promise: Promise<T>,
    options: {
      loading: string;
      success: string | ((data: T) => string);
      error: string | ((error: unknown) => string);
    },
  ) => toast.promise(promise, options),
};

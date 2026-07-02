import { toast } from '@heroui/react/toast';
import { ApiError } from '@/shared/api/errors';

/**
 * Project wrapper around Hero UI `toast`.
 * Use `notify` everywhere instead of importing `toast` directly so that
 * timeouts, error formatting, and Russian copy stay consistent.
 */

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
    return 'Произошла ошибка';
  }
  if (typeof error === 'string') {
    return error;
  }
  return 'Произошла ошибка';
}

function error(title: string, options?: ToastOptions): string;
function error(error: unknown, options?: ToastOptions): string;
function error(titleOrError: string | unknown, options?: ToastOptions): string {
  const title =
    typeof titleOrError === 'string'
      ? titleOrError
      : normalizeError(titleOrError);
  return toast.danger(title, {
    description: options?.description,
    timeout: options?.duration ?? DEFAULT_DURATIONS.error,
  });
}

export const notify = {
  success: (title: string, options?: ToastOptions): string =>
    toast.success(title, {
      description: options?.description,
      timeout: options?.duration ?? DEFAULT_DURATIONS.success,
    }),

  error,

  info: (title: string, options?: ToastOptions): string =>
    toast.info(title, {
      description: options?.description,
      timeout: options?.duration ?? DEFAULT_DURATIONS.info,
    }),

  warning: (title: string, options?: ToastOptions): string =>
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
  ): string => toast.promise(promise, options),
};

import { createElement } from 'react';
import { toast } from 'react-toastify/unstyled';
import { ApiError } from '@/shared/api/errors';
import {
  ToastBody,
  type ToastBodyProps,
  type ToastVariant,
} from '@/shared/ui/toast/ToastBody';
import { CloseToastButton } from '@/shared/ui/toast/ToastProvider';
import type {
  NotificationKey,
  ScenarioOptions,
} from './types';

const DEFAULT_DURATIONS = {
  success: 4000,
  error: 4000,
  info: 4000,
  warning: 4000,
} as const;

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

function toastContent(
  variant: ToastVariant,
  title: string,
  options?: ScenarioOptions,
): ToastBodyProps {
  return {
    variant,
    title,
    description: options?.description,
    action: options?.action,
  };
}

function error(title: string, options?: ScenarioOptions): NotificationKey;
function error(error: unknown, options?: ScenarioOptions): NotificationKey;
function error(
  titleOrError: string | unknown,
  options?: ScenarioOptions,
): NotificationKey {
  const title =
    typeof titleOrError === 'string'
      ? titleOrError
      : normalizeError(titleOrError);
  return String(
    toast.error(
      ({ closeToast }) =>
        createElement(ToastBody, {
          ...toastContent('error', title, options),
          closeToast,
        }),
      {
        autoClose: options?.duration ?? DEFAULT_DURATIONS.error,
      },
    ),
  );
}

export type NotifyBase = {
  success(title: string, options?: ScenarioOptions): NotificationKey;
  error(title: string, options?: ScenarioOptions): NotificationKey;
  error(error: unknown, options?: ScenarioOptions): NotificationKey;
  info(title: string, options?: ScenarioOptions): NotificationKey;
  warning(title: string, options?: ScenarioOptions): NotificationKey;
  loading(title: string, options?: ScenarioOptions): NotificationKey;
  close(key: string): void;
  promise<T>(
    promise: Promise<T>,
    options: {
      loading: string;
      success: string | ((data: T) => string);
      error: string | ((error: unknown) => string);
    },
  ): NotificationKey;
};

let promiseToastCounter = 0;

export const notify: NotifyBase = {
  success: (title, options): NotificationKey =>
    String(
      toast.success(
        ({ closeToast }) =>
          createElement(ToastBody, {
            ...toastContent('success', title, options),
            closeToast,
          }),
        { autoClose: options?.duration ?? DEFAULT_DURATIONS.success },
      ),
    ),

  error,

  info: (title, options): NotificationKey =>
    String(
      toast.info(
        ({ closeToast }) =>
          createElement(ToastBody, {
            ...toastContent('info', title, options),
            closeToast,
          }),
        { autoClose: options?.duration ?? DEFAULT_DURATIONS.info },
      ),
    ),

  warning: (title, options): NotificationKey =>
    String(
      toast.warning(
        ({ closeToast }) =>
          createElement(ToastBody, {
            ...toastContent('warning', title, options),
            closeToast,
          }),
        { autoClose: options?.duration ?? DEFAULT_DURATIONS.warning },
      ),
    ),

  // toast.loading hides the close button and disables dragging by default;
  // both are re-enabled to match the previous behavior.
  loading: (title, options): NotificationKey =>
    String(
      toast.loading(
        ({ closeToast }) =>
          createElement(ToastBody, {
            ...toastContent('loading', title, options),
            closeToast,
          }),
        {
          autoClose: false,
          closeButton: CloseToastButton,
          draggable: true,
        },
      ),
    ),

  close: (key): void => toast.dismiss(key),

  // toast.promise resolves with the promise result instead of the toast id,
  // so the id is generated up front and passed via `toastId`.
  promise: <T,>(
    promise: Promise<T>,
    options: {
      loading: string;
      success: string | ((data: T) => string);
      error: string | ((error: unknown) => string);
    },
  ): NotificationKey => {
    promiseToastCounter += 1;
    const key = `promise-${promiseToastCounter}`;

    void toast.promise<T>(
      promise,
      {
        pending: {
          render: ({ closeToast }) =>
            createElement(ToastBody, {
              variant: 'loading',
              title: options.loading,
              closeToast,
            }),
          closeButton: CloseToastButton,
          draggable: true,
        },
        success: {
          render: ({ data, closeToast }) =>
            createElement(ToastBody, {
              variant: 'success',
              title:
                typeof options.success === 'function'
                  ? options.success(data)
                  : options.success,
              closeToast,
            }),
          autoClose: DEFAULT_DURATIONS.success,
          closeButton: CloseToastButton,
        },
        error: {
          render: ({ data, closeToast }) =>
            createElement(ToastBody, {
              variant: 'error',
              title:
                typeof options.error === 'function'
                  ? options.error(data)
                  : options.error,
              closeToast,
            }),
          autoClose: DEFAULT_DURATIONS.error,
          closeButton: CloseToastButton,
        },
      },
      { toastId: key },
    );

    return key;
  },
};

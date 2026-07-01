'use client';

import type { JSX } from 'react';
import { Toast, Spinner, type ToastContentValue } from '@heroui/react';
import type { QueuedToast } from 'react-aria-components';
import './toast.css';

function renderToast({ toast }: { toast: QueuedToast<ToastContentValue> }): JSX.Element {
  const { actionProps, description, indicator, isLoading, title, variant } = toast.content ?? {};
  const dataVariant = variant === 'accent' ? 'info' : variant ?? 'default';

  return (
    <Toast
      toast={toast}
      variant={variant}
      className="toast-base toast-wrapper"
      data-variant={dataVariant}
    >
      {indicator === null ? null : (
        <Toast.Indicator variant={variant} className="toast-icon">
          {isLoading ? <Spinner color="current" size="sm" /> : indicator}
        </Toast.Indicator>
      )}
      <Toast.Content className="toast-content">
        {!!title && <Toast.Title className="toast-title">{title}</Toast.Title>}
        {!!description && (
          <Toast.Description className="toast-description">{description}</Toast.Description>
        )}
        {actionProps?.children ? (
          <Toast.ActionButton {...actionProps}>{actionProps.children}</Toast.ActionButton>
        ) : null}
      </Toast.Content>
      <Toast.CloseButton className="toast-close-button" />
    </Toast>
  );
}

export function ToastProvider(): JSX.Element {
  return (
    <Toast.Provider
      placement="bottom end"
      maxVisibleToasts={4}
      className="toast-region"
    >
      {renderToast}
    </Toast.Provider>
  );
}

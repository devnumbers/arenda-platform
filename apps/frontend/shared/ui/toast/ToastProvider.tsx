'use client';

import { type JSX, useEffect, useState } from 'react';
import { Toast, Spinner, type ToastContentValue } from '@heroui/react';
import type { QueuedToast } from 'react-aria-components';
import clsx from 'clsx';
import styles from './ToastProvider.module.css';

const MOBILE_MEDIA_QUERY = '(max-width: 767px)';

function useIsMobile(): boolean {
  const [isMobile, setIsMobile] = useState(false);

  useEffect(() => {
    const mql = window.matchMedia(MOBILE_MEDIA_QUERY);
    const update = (matches: boolean) => setIsMobile(matches);

    update(mql.matches);

    const onChange = (event: MediaQueryListEvent) => update(event.matches);
    mql.addEventListener('change', onChange);
    return () => mql.removeEventListener('change', onChange);
  }, []);

  return isMobile;
}

function renderToast({ toast }: { toast: QueuedToast<ToastContentValue> }): JSX.Element {
  const { actionProps, description, indicator, isLoading, title, variant } = toast.content ?? {};

  // Hero UI uses the `accent` variant for info toasts; normalize it to `info`
  // so the project CSS data attribute matches the public toast API.
  const dataVariant = variant === 'accent' ? 'info' : variant ?? 'default';

  return (
    <Toast
      toast={toast}
      variant={variant}
      className={clsx(styles.toast, styles.wrapper)}
      data-variant={dataVariant}
    >
      {(indicator !== null || isLoading) && (
        <Toast.Indicator variant={variant} className={styles.icon}>
          {isLoading ? <Spinner color="current" size="sm" /> : indicator}
        </Toast.Indicator>
      )}
      <Toast.Content className={styles.content}>
        {!!title && <Toast.Title className={styles.title}>{title}</Toast.Title>}
        {!!description && (
          <Toast.Description className={styles.description}>{description}</Toast.Description>
        )}
        {actionProps?.children && (
          <Toast.ActionButton className={styles.actionButton} {...actionProps} />
        )}
      </Toast.Content>
      <Toast.CloseButton className={styles.closeButton} />
    </Toast>
  );
}

export function ToastProvider(): JSX.Element {
  const isMobile = useIsMobile();

  return (
    <Toast.Provider
      placement={isMobile ? 'top' : 'bottom end'}
      maxVisibleToasts={4}
      className={styles.region}
    >
      {renderToast}
    </Toast.Provider>
  );
}

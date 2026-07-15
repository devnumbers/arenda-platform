'use client';

import { useCallback, useEffect, useRef, useState, type JSX } from 'react';
import clsx from 'clsx';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@heroui/react/skeleton';
import { Modal } from '@heroui/react';
import { notify } from '@/shared/lib/notifications';
import { Button } from '@/shared/ui/button';
import { LinkButton } from '@/shared/ui/link-button';
import {
  usePaymentMethods,
  useActivatePaymentMethod,
  useDeletePaymentMethod,
  useSyncPaymentMethods,
} from '@/features/billing/api/hooks';
import { ROUTES } from '@/shared/config/routes';
import { formatDate } from '@/shared/lib/format-date';
import { ApiError } from '@/shared/api/errors';
import styles from './PaymentMethodList.module.css';

type DeleteModalProps = {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onConfirm: () => void;
  readonly displayMask?: string;
  readonly isLoading: boolean;
};

function DeletePaymentMethodModal({
  isOpen,
  onClose,
  onConfirm,
  displayMask,
  isLoading,
}: DeleteModalProps): JSX.Element {
  const handleOpenChange = (open: boolean): void => {
    if (!open) {
      onClose();
    }
  };

  return (
    <Modal>
      <Modal.Backdrop isOpen={isOpen} onOpenChange={handleOpenChange}>
        <Modal.Container placement="center" size="sm">
          <Modal.Dialog aria-label="Удалить карту">
            <Modal.Header>
              <Modal.Heading>Удалить карту?</Modal.Heading>
            </Modal.Header>
            <Modal.Body>
              <p>
                {displayMask
                  ? `Карта ${displayMask} будет удалена. Платежи по ней больше не будут проводиться.`
                  : 'Карта будет удалена. Платежи по ней больше не будут проводиться.'}
              </p>
            </Modal.Body>
            <Modal.Footer>
              <Button variant="secondary" onClick={onClose} type="button">
                Отменить
              </Button>
              <Button
                variant="primary"
                onClick={onConfirm}
                type="button"
                loading={isLoading}
              >
                Удалить
              </Button>
            </Modal.Footer>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}

function PaymentMethodListSkeleton(): JSX.Element {
  return (
    <div className={styles.list}>
      {[1, 2].map((key) => (
        <Card key={key} className={styles.card}>
          <Skeleton className={styles.maskSkeleton} />
          <Skeleton className={styles.metaSkeleton} />
        </Card>
      ))}
    </div>
  );
}

type PaymentMethodListProps = {
  readonly addCardResult?: string;
};

export function PaymentMethodList({
  addCardResult,
}: PaymentMethodListProps): JSX.Element {
  const {
    data: items,
    isPending,
    isError,
    refetch,
  } = usePaymentMethods();
  const activate = useActivatePaymentMethod();
  const remove = useDeletePaymentMethod();
  const sync = useSyncPaymentMethods();
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const hasAutoSyncedRef = useRef(false);

  useEffect(() => {
    if (addCardResult === 'success') {
      notify.scenarios.paymentMethods.cardAdded();
      void refetch();
    } else if (addCardResult === 'fail') {
      notify.scenarios.paymentMethods.cardAddError();
    }

    if (addCardResult === 'success' || addCardResult === 'fail') {
      const url = new URL(window.location.href);
      url.searchParams.delete('addCard');
      window.history.replaceState({}, '', url.toString());
    }
  }, [addCardResult, refetch]);

  const notifySyncError = useCallback((error: ApiError) => {
    notify.scenarios.paymentMethods.listUpdateWarning({description: error.detail});
  }, []);

  const handleSync = useCallback(() => {
    sync.mutate(undefined, { onError: notifySyncError });
  }, [sync, notifySyncError]);

  // Sync with the provider once on mount, so a card linked on the bank
  // form appears without a manual refresh. The ref guards against the
  // StrictMode double effect run in dev; the mutation is idempotent anyway.
  useEffect(() => {
    if (hasAutoSyncedRef.current) return;
    hasAutoSyncedRef.current = true;
    handleSync();
  }, [handleSync]);

  const handleActivate = useCallback(
    (id: string) => {
      void notify.scenarios.paymentMethods.activated(activate.mutateAsync(id));
    },
    [activate],
  );

  const handleConfirmDelete = useCallback(() => {
    if (!deletingId) {
      return;
    }

    const promise = remove.mutateAsync(deletingId);

    void notify.scenarios.paymentMethods.removed(promise);

    promise
      .finally(() => {
        setDeletingId(null);
      })
      .catch(() => {}); // error is already reported by notify.promise
  }, [deletingId, remove]);

  const deletingMethod = items?.find((method) => method.id === deletingId);
  const isMutating = activate.isPending || remove.isPending;

  if (isError) {
    return (
      <div className={styles.error}>
        <p className={styles.errorText}>
          Не удалось загрузить способы оплаты
        </p>
        <Button onClick={() => refetch()} variant="secondary">
          Повторить
        </Button>
      </div>
    );
  }

  if (isPending || !items) {
    return <PaymentMethodListSkeleton />;
  }

  if (items.length === 0) {
    return (
      <div className={styles.empty}>
        <p className={styles.emptyText}>У вас пока нет сохранённых карт</p>
        <LinkButton
          href={ROUTES.profilePaymentMethodsAdd}
          variant="primary"
          size="large"
          fullWidth
        >
          Добавить карту
        </LinkButton>
        <Button
          variant="secondary"
          size="small"
          onClick={handleSync}
          loading={sync.isPending}
        >
          Обновить
        </Button>
      </div>
    );
  }

  return (
    <div className={styles.root}>
      <div className={styles.toolbar}>
        <Button
          variant="secondary"
          size="small"
          onClick={handleSync}
          loading={sync.isPending}
        >
          Обновить
        </Button>
      </div>
      <div className={styles.list}>
        {items.map((method) => (
          <Card
            key={method.id}
            className={clsx(
              styles.card,
              method.isActive && styles.activeCard,
            )}
          >
            <div className={styles.cardHeader}>
              <span className={styles.mask}>{method.displayMask}</span>
              {method.isActive && (
                <span className={styles.activeBadge}>Основная</span>
              )}
            </div>

            <p className={styles.meta}>
              {method.provider}
              {method.createdAt && (
                <span className={styles.date}>
                  {' · Добавлена '}
                  {formatDate(method.createdAt)}
                </span>
              )}
            </p>

            {!method.isActive && (
              <div className={styles.actions}>
                <Button
                  variant="secondary"
                  size="small"
                  onClick={() => handleActivate(method.id)}
                  loading={activate.isPending}
                  disabled={isMutating}
                >
                  Сделать основной
                </Button>
                <Button
                  variant="clear"
                  size="small"
                  className={styles.deleteButton}
                  onClick={() => setDeletingId(method.id)}
                  disabled={isMutating}
                >
                  Удалить
                </Button>
              </div>
            )}
          </Card>
        ))}
      </div>

      <LinkButton
        href={ROUTES.profilePaymentMethodsAdd}
        variant="primary"
        size="large"
        fullWidth
      >
        Добавить карту
      </LinkButton>

      <DeletePaymentMethodModal
        isOpen={Boolean(deletingId)}
        onClose={() => setDeletingId(null)}
        onConfirm={handleConfirmDelete}
        displayMask={deletingMethod?.displayMask}
        isLoading={remove.isPending}
      />
    </div>
  );
}

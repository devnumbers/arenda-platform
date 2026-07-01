'use client';

import { useCallback, useEffect, useState, type JSX } from 'react';
import clsx from 'clsx';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@heroui/react/skeleton';
import { Modal } from '@heroui/react';
import { notify } from '@/shared/lib/toast';
import { Button } from '@/shared/ui/button';
import { LinkButton } from '@/shared/ui/link-button';
import {
  usePaymentMethods,
  useActivatePaymentMethod,
  useDeletePaymentMethod,
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
  const [deletingId, setDeletingId] = useState<string | null>(null);

  useEffect(() => {
    if (addCardResult === 'success') {
      notify.success('Карта успешно добавлена');
      void refetch();
    } else if (addCardResult === 'fail') {
      notify.error('Не удалось добавить карту. Попробуйте снова.');
    }

    if (addCardResult === 'success' || addCardResult === 'fail') {
      const url = new URL(window.location.href);
      url.searchParams.delete('addCard');
      window.history.replaceState({}, '', url.toString());
    }
  }, [addCardResult, refetch]);

  const handleActivate = useCallback(
    (id: string) => {
      void notify.promise(activate.mutateAsync(id), {
        loading: 'Делаем карту основной...',
        success: 'Карта стала основной',
        error: (error) =>
          (error as ApiError).detail ?? 'Не удалось сделать карту основной',
      });
    },
    [activate],
  );

  const handleConfirmDelete = useCallback(() => {
    if (!deletingId) {
      return;
    }

    const promise = remove.mutateAsync(deletingId);

    void notify.promise(promise, {
      loading: 'Удаляем карту...',
      success: 'Карта удалена',
      error: (error) =>
        (error as ApiError).detail ?? 'Не удалось удалить карту',
    });

    promise.finally(() => {
      setDeletingId(null);
    });
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
      </div>
    );
  }

  return (
    <div className={styles.root}>
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

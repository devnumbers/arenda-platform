'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import {
  Button,
  ConfirmDialog,
  IconButton,
  Modal,
  ModalContent,
  RadioGroup,
  RadioGroupItem,
  Skeleton,
} from '@/shared/ui/design';
import { Add, CheckmarkCircle, Info, TrashBin } from '@/shared/assets/icons';
import { notify } from '@/shared/lib/notifications';
import {
  useActivatePaymentMethod,
  useAddPaymentMethod,
  useDeletePaymentMethod,
  usePaymentMethods,
  useSyncPaymentMethods,
} from '@/features/billing';
import type { ApiError } from '@/shared/api/errors';
import type { PaymentMethod } from '@/entities/billing';
import {
  paymentMethodDeleteGuard,
  paymentMethodTitle,
} from '../lib/payment-methods';

/** Тексты гардов удаления (#625): активная карта обслуживает
 * автопродление (бэк отвечает 409 in use), единственную нечем заменить. */
const DELETE_GUARD_COPY: Record<'active' | 'only', { title: string; description: string }> = {
  active: {
    title: 'Нельзя удалить активный способ оплаты',
    description:
      'Активная карта используется для автопродления подписки. Сначала выберите другой способ оплаты',
  },
  only: {
    title: 'Нельзя удалить единственный способ оплаты',
    description: 'Чтобы удалить карту, сначала привяжите другой способ оплаты',
  },
};

/** Серый баннер под шапкой (макеты 1879-71079 / 1886-109725): с картами —
 * про списание за тариф с канон-иконкой Checkmark; без карт текст
 * меняется на «нет сохраненных способов оплаты» с Icon/R/Info. */
function MethodsBanner({ empty }: { readonly empty: boolean }): JSX.Element {
  return (
    <div className="flex items-center gap-2 rounded-[24px] bg-surface-muted px-6 py-5">
      {empty ? (
        <Info className="h-6 w-6 shrink-0" aria-hidden />
      ) : (
        <CheckmarkCircle className="h-6 w-6 shrink-0" aria-hidden />
      )}
      <p className="m-0 text-base font-medium leading-[18px] text-content">
        {empty
          ? 'У вас нет сохраненных способов оплаты'
          : 'С выбранной карты будет списываться оплата за тариф'}
      </p>
    </div>
  );
}

function MethodsSkeleton(): JSX.Element {
  return (
    <div
      className="flex flex-col gap-4"
      role="status"
      aria-label="Загружаем способы оплаты"
    >
      <Skeleton className="h-[76px] rounded-[24px]" />
      {[0, 1].map((row) => (
        <div key={row} className="flex min-h-[68px] items-center gap-3 px-3 py-3">
          <span className="flex h-11 w-11 shrink-0 items-center justify-center">
            <Skeleton className="h-6 w-6 rounded-full" />
          </span>
          <Skeleton className="h-[18px] w-44" />
          <span className="flex-1" />
          <Skeleton className="h-6 w-6" />
        </div>
      ))}
      <div className="flex min-h-[68px] items-center gap-3 px-3 py-3">
        <Skeleton className="h-6 w-6" />
        <Skeleton className="h-[18px] w-56" />
      </div>
    </div>
  );
}

type PaymentMethodListProps = {
  readonly addCardResult?: string;
};

/** Экран «Способы оплаты» (#625, макеты 1879-71079 список, 1886-109725
 * пусто): баннер, радио-строки карт («•••• 0700» — бренд платёжной
 * системы не выводим, решение владельца 15.09 #611; лейбл банка не
 * выводится — решение владельца #614; активная = выбранное радио, тап по
 * строке активирует), корзина с подтверждением и двумя гардами, строка
 * «Добавить способ оплаты» в flow привязки Т-Кассы (confirmUrl, возврат
 * на экран с ?addCard=success|fail). Синк с провайдером — раз при
 * монтировании (#251). Старый HeroUI-список с кнопками «Сделать
 * основной»/«Обновить» заменён целиком. */
export function PaymentMethodList({
  addCardResult,
}: PaymentMethodListProps): JSX.Element {
  const {
    data,
    isPending,
    isError,
    refetch,
  } = usePaymentMethods();
  const activate = useActivatePaymentMethod();
  const remove = useDeletePaymentMethod();
  const sync = useSyncPaymentMethods();
  const add = useAddPaymentMethod();
  const methods: PaymentMethod[] = data ?? [];
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [deleteGuard, setDeleteGuard] = useState<'active' | 'only' | null>(null);
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

  // Синк с провайдером один раз при монтировании: карта, привязанная в
  // банковской форме без возврата на экран, появляется сама. Ref гасит
  // повторный запуск StrictMode в dev; мутация идемпотентна.
  useEffect(() => {
    if (hasAutoSyncedRef.current) return;
    hasAutoSyncedRef.current = true;
    sync.mutate(undefined, {
      onError: (error: ApiError) => {
        notify.scenarios.paymentMethods.listUpdateWarning({
          description: error.detail,
        });
      },
    });
  }, [sync]);

  const handleActivate = (id: string): void => {
    void notify.scenarios.paymentMethods.activated(activate.mutateAsync(id));
  };

  const handleTrash = (method: PaymentMethod): void => {
    // Кнопка внутри label-строки: preventDefault гасит активацию радио
    // (default action лейбла), иначе тап по корзине меняет активную карту.
    const guard = paymentMethodDeleteGuard(method, methods.length);
    if (guard !== undefined) {
      setDeleteGuard(guard);
      return;
    }
    setDeletingId(method.id);
  };

  const handleConfirmDelete = (): void => {
    if (deletingId === null) {
      return;
    }

    const promise = remove.mutateAsync(deletingId);

    void notify.scenarios.paymentMethods.removed(promise);

    promise
      .finally(() => {
        setDeletingId(null);
      })
      .catch(() => {}); // ошибка уже показана тостом сценария
  };

  const handleAdd = (): void => {
    add.mutate(
      {},
      {
        onSuccess: (result) => {
          if (result.confirmUrl !== undefined) {
            // Банковская форма привязки — уход с экрана; возврат на
            // /profile/tariff/payment-methods?addCard=success|fail.
            window.location.assign(result.confirmUrl);
            return;
          }
          notify.scenarios.paymentMethods.cardAddError();
        },
        onError: (error: ApiError) => {
          notify.scenarios.auth.addCardStartError({ description: error.detail });
        },
      },
    );
  };

  if (isError) {
    return (
      <div className="flex flex-col items-center gap-4 py-16 text-center">
        <p className="m-0 text-base leading-[18px] text-content-secondary">
          Не удалось загрузить способы оплаты
        </p>
        <Button onClick={() => void refetch()} variant="secondary">
          Повторить
        </Button>
      </div>
    );
  }

  if (isPending) {
    return <MethodsSkeleton />;
  }

  const activeId = methods.find((method) => method.isActive)?.id;

  return (
    <>
      <MethodsBanner empty={methods.length === 0} />

      <div className="mt-4 flex flex-col">
        <RadioGroup
          value={activeId}
          onValueChange={handleActivate}
          className="flex flex-col"
        >
          {methods.map((method) => (
            <label
              key={method.id}
              className="flex min-h-[68px] cursor-pointer items-center gap-3 px-3 py-3"
            >
              <span className="flex h-11 w-11 shrink-0 items-center justify-center">
                <RadioGroupItem value={method.id} className="h-6 w-6" />
              </span>
              <span className="min-w-0 flex-1 truncate text-base font-medium leading-[18px] text-content">
                {paymentMethodTitle(method)}
              </span>
              <IconButton
                variant="primary"
                icon={<TrashBin className="h-6 w-6" />}
                label={`Удалить карту ${paymentMethodTitle(method)}`}
                onClick={(event) => {
                  event.preventDefault();
                  handleTrash(method);
                }}
              />
            </label>
          ))}
        </RadioGroup>

        <button
          type="button"
          data-testid="add-payment-method"
          onClick={handleAdd}
          disabled={add.isPending}
          className="flex min-h-[68px] w-full cursor-pointer items-center gap-3 px-3 py-3 text-left font-sans outline-none transition-opacity hover:opacity-80 active:opacity-80 disabled:pointer-events-none disabled:opacity-50 focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2"
        >
          <span className="flex h-11 w-11 shrink-0 items-center justify-center">
            <Add className="h-6 w-6" aria-hidden />
          </span>
          <span className="text-base font-medium leading-[18px] text-content">
            Добавить способ оплаты
          </span>
        </button>
      </div>

      <ConfirmDialog
        open={deletingId !== null}
        onOpenChange={(open) => {
          if (!open) {
            setDeletingId(null);
          }
        }}
        title="Уверены, что хотите удалить способ оплаты?"
        description="Карта будет удалена"
        confirmLabel="Удалить"
        confirmVariant="danger"
        pending={remove.isPending}
        onConfirm={handleConfirmDelete}
      />

      <Modal
        open={deleteGuard !== null}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteGuard(null);
          }
        }}
      >
        <ModalContent
          title={
            deleteGuard === null ? undefined : DELETE_GUARD_COPY[deleteGuard].title
          }
          description={
            deleteGuard === null
              ? undefined
              : DELETE_GUARD_COPY[deleteGuard].description
          }
        >
          <Button className="w-full" onClick={() => setDeleteGuard(null)}>
            Понятно
          </Button>
        </ModalContent>
      </Modal>
    </>
  );
}

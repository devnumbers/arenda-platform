'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { useLeaveProperty } from '@/features/participants';
import { type SuspendedSharedProperty } from '@/features/properties';
import { BoldUser, Exit, Lock } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { Button, Modal, ModalContent } from '@/shared/ui/design';

export type SuspendedReasonSheetProps = {
  readonly placeholder: SuspendedSharedProperty | null;
  readonly onClose: () => void;
};

/**
 * Шит причины подвесшего объекта (карта #692, тикет #702; Figma
 * 2229-100002): замок, «Превышен лимит объектов», путь к доступу, ряд
 * владельца (имя + почта — сознательная экспозиция этого экрана по макету)
 * и три действия: красный выход «Покинуть объект» (DELETE members/self —
 * с suspended-доступа теперь можно выйти, #702), «Выбрать тариф» (CTA на
 * смену тарифа) и «Закрыть». Успех выхода без попапа — макет его не рисует,
 * плейсхолдер уходит после инвалидации (useLeaveProperty, #701).
 */
export function SuspendedReasonSheet({
  placeholder,
  onClose,
}: SuspendedReasonSheetProps): JSX.Element {
  const router = useRouter();
  const leave = useLeaveProperty();

  const leaveProperty = (): void => {
    if (placeholder === null) {
      return;
    }
    leave.mutate(placeholder.propertyId, {
      onSuccess: onClose,
    });
  };

  const chooseTariff = (): void => {
    onClose();
    router.push(ROUTES.profileTariffChange);
  };

  return (
    <Modal
      open={placeholder !== null}
      onOpenChange={(open) => {
        if (!open) {
          onClose();
        }
      }}
    >
      <ModalContent title="Превышен лимит объектов" titleSrOnly>
        {placeholder !== null && (
          <div className="flex flex-col gap-8" data-testid="suspended-reason-sheet">
            <div className="flex flex-col items-center gap-4">
              <Lock className="h-6 w-6 text-content" aria-hidden />
              <div className="flex flex-col gap-1">
                <p className="m-0 text-center text-xl font-semibold leading-6 text-content">
                  Превышен лимит объектов
                </p>
                <p className="m-0 text-center text-sm leading-4 text-content-secondary">
                  Вас пригласили в объект. Чтобы получить доступ, удалите
                  остальные объекты, переведите их в архив, или выберите тариф
                  с большим количеством объектов
                </p>
              </div>
            </div>

            <div className="flex flex-col gap-2">
              <p className="m-0 pl-2.5 text-[13px] leading-[15px] text-content-tertiary">
                Владелец объекта
              </p>
              <div className="flex items-center gap-2">
                <span
                  aria-hidden
                  className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-surface-muted"
                >
                  <BoldUser className="h-[13px] w-[13px]" />
                </span>
                <span className="flex min-w-0 flex-col">
                  <span className="truncate text-[13px] font-medium leading-[15px] text-content">
                    {placeholder.ownerName}
                  </span>
                  {placeholder.ownerEmail !== '' && (
                    <span className="truncate text-[13px] leading-[15px] text-content-secondary">
                      {placeholder.ownerEmail}
                    </span>
                  )}
                </span>
              </div>
              <button
                type="button"
                onClick={leaveProperty}
                disabled={leave.isPending}
                data-testid="suspended-leave"
                className="flex w-full cursor-pointer items-center gap-3 px-1 py-1 text-left outline-none transition-opacity hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface active:opacity-80 disabled:cursor-default disabled:opacity-60"
              >
                <Exit className="h-6 w-6 shrink-0 text-error" aria-hidden />
                <span className="text-base font-medium leading-[18px] text-error">
                  Покинуть объект
                </span>
              </button>
            </div>

            <div className="flex flex-col gap-2">
              <Button
                variant="primary"
                onClick={chooseTariff}
                data-testid="suspended-choose-tariff"
                className="w-full"
              >
                Выбрать тариф
              </Button>
              <Button
                variant="secondary"
                onClick={onClose}
                data-testid="suspended-close"
                className="w-full"
              >
                Закрыть
              </Button>
            </div>
          </div>
        )}
      </ModalContent>
    </Modal>
  );
}

'use client';

import type { JSX } from 'react';
import Image from 'next/image';
import { Button, Modal, ModalContent } from '@/shared/ui/design';

/** Шит «Разрешите пуши» (макет 2329-151632): открывается при попытке
 * включить пуш-тумблер без разрешения браузера. «Разрешить» ведёт флоу
 * разрешения (#746) с ожидающим изменением; «Не разрешать» (как и свайп/тап
 * по оверлею) закрывает шит, тумблер не двигается. На планшете/ПК канон
 * Modal рисует карточку вместо шита. */
export function NotificationPermissionSheet({
  open,
  pending,
  onAllow,
  onDecline,
}: {
  readonly open: boolean;
  readonly pending: boolean;
  readonly onAllow: () => void;
  readonly onDecline: () => void;
}): JSX.Element {
  return (
    <Modal
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          onDecline();
        }
      }}
    >
      <ModalContent
        title={
          <span className="flex w-full items-start justify-between gap-4">
            Разрешите пуши
            <Image
              src="/images/notifications/empty-bell.png"
              alt=""
              width={56}
              height={56}
              className="h-14 w-14 shrink-0"
              unoptimized
            />
          </span>
        }
        description="Разрешите пуш-уведомления в браузере, чтобы получать уведомления"
      >
        <div className="flex gap-2">
          <Button
            variant="white"
            className="flex-1"
            onClick={onDecline}
            disabled={pending}
          >
            Не разрешать
          </Button>
          <Button
            variant="primary"
            className="flex-1"
            onClick={onAllow}
            disabled={pending}
          >
            {pending ? 'Разрешаем…' : 'Разрешить'}
          </Button>
        </div>
      </ModalContent>
    </Modal>
  );
}

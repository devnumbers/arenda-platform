'use client';

import type { JSX } from 'react';
import Image from 'next/image';
import { Button, Modal, ModalContent } from '@/shared/ui/design';

/** Шит «Разрешите пуши» (макеты 2329-151632 / 2329-151938): слева колонка
 * заголовок 20/24 + описание 14/16 (зазор 4), справа колокольчик 64×64;
 * ниже — равные половины «Не разрешать» (secondary) и «Разрешить» (primary)
 * с зазором 4. Открывается при попытке включить пуш-тумблер без разрешения
 * браузера. «Разрешить» ведёт флоу разрешения (#746) с ожидающим изменением;
 * «Не разрешать» (как и свайп/тап по оверлею) закрывает шит, тумблер не
 * двигается. На планшете/ПК канон Modal рисует карточку с той же
 * композицией. */
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
      {/* a11y-имя диалога — sr-only DialogTitle слота title; видимая
       * композиция (двухколонник текст+белл) собрана в children, слоты
       * title/description её форму не дают. */}
      <ModalContent title="Разрешите пуши" titleSrOnly>
        <div className="flex items-start gap-4">
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <p className="m-0 text-xl font-semibold leading-6 text-content">Разрешите пуши</p>
            <p className="m-0 text-sm leading-4 text-content-secondary">
              Разрешите пуш-уведомления в браузере, чтобы получать уведомления
            </p>
          </div>
          <Image
            src="/images/notifications/empty-bell.png"
            alt=""
            width={64}
            height={64}
            className="h-16 w-16 shrink-0"
            unoptimized
          />
        </div>
        {/* Зазор текст↔кнопки 24: 16 даёт gap-4 контейнера ModalContent,
         * ещё 8 — этот mt-2 (макет 2329-151938). */}
        <div className="mt-2 flex gap-1">
          <Button
            variant="secondary"
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

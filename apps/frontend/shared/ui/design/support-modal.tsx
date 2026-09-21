'use client';

import { type JSX } from 'react';
import { Cancel, Check, Copy } from '@/shared/assets/icons';
import { SUPPORT_EMAIL, SUPPORT_TELEGRAM_URL } from '@/shared/config/support';
import { useCopiedHint } from '@/shared/lib/hooks/use-copied-hint';
import { Button, buttonVariants } from './button';
import { HeaderLogo } from './header-logo';
import { IconButton } from './icon-button';
import { Modal, ModalClose, ModalContent, useIsDesktop } from './modal';

export type SupportModalProps = {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
};

/**
 * Модалка «Связаться с нами» — единая поверхность поддержки вместо страницы
 * /support (карта #761, тикет #766; решение владельца 18.09). Канвы Figma
 * «Рентли. Новые экраны сервиса»: 2355:52709 — десктоп (центрированная
 * карточка), 2355:52684 — планшет 768 (та же карточка), 2355:52746 —
 * мобайл (боттом-шит) — всё это канва Modal (карточка ≥768 / vaul-шит
 * ниже), поэтому компонент только наполняет ModalContent. Состав макета:
 * лого 112×28 (HeaderLogo), заголовок H1 28/32 + подпись 16/18 (зазор 8),
 * Primary «Написать в Телеграм» (внешняя ссылка в новой вкладке) и
 * Secondary-строка почты с копированием в буфер (зазор 8) — блоки с
 * зазорами 24/32. Карточка макета уже канона (≈400 против 520) —
 * переопределение max-w. Крестик — только в карточке (в шите закрытие
 * свайпом/оверлеем, как у канвы Modal). Контакты — константы
 * shared/config/support.ts, литералов здесь нет.
 */
export function SupportModal({ open, onOpenChange }: SupportModalProps): JSX.Element {
  const isDesktop = useIsDesktop();
  const { copied, copy } = useCopiedHint();

  return (
    <Modal open={open} onOpenChange={onOpenChange}>
      <ModalContent title="Связаться с нами" titleSrOnly className="max-w-[400px]">
        <div className="flex flex-col gap-6">
          <div className="flex items-start justify-between gap-4">
            <HeaderLogo className="h-7 w-28" />
            {isDesktop && (
              <ModalClose asChild>
                <IconButton icon={<Cancel />} label="Закрыть" />
              </ModalClose>
            )}
          </div>

          <div className="flex flex-col gap-8">
            <div className="flex flex-col gap-2">
              {/* A11y-имя диалога даёт sr-only DialogTitle; видимый заголовок
               * канвы — дубликат, от скринридеров скрыт. */}
              <h2 aria-hidden className="m-0 text-[28px] font-semibold leading-8 text-content">
                Связаться с нами
              </h2>
              <p className="m-0 text-base leading-[18px] text-content-secondary">
                Напишите нам в Телеграм или на почту
              </p>
            </div>

            <div className="flex flex-col gap-2">
              {/* Внешняя ссылка на канонной кнопке: next/link не дружит со
               * Slot — ссылки стилизуются buttonVariants (кнопка.tsx). */}
              <a
                href={SUPPORT_TELEGRAM_URL}
                target="_blank"
                rel="noopener noreferrer"
                className={buttonVariants()}
              >
                Написать в Телеграм
              </a>
              <Button
                type="button"
                variant="secondary"
                trailingIcon={
                  copied ? <Check className="h-6 w-6" aria-hidden /> : <Copy className="h-6 w-6" aria-hidden />
                }
                aria-label={copied ? 'Скопировано' : `Скопировать: ${SUPPORT_EMAIL}`}
                onClick={() => copy(SUPPORT_EMAIL)}
              >
                {SUPPORT_EMAIL}
              </Button>
            </div>
          </div>
        </div>
      </ModalContent>
    </Modal>
  );
}

'use client';

import type { JSX } from 'react';
import {
  DialogClose,
  DialogContent,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  Dialog as DialogRoot,
} from '@radix-ui/react-dialog';
import { Cancel, StatusIconGood } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { IconButton } from './icon-button';

/**
 * Успех-попап дизайн-слоя (#744, Figma 2329:148661 — карточка варианта
 * Popup): центрированная белая карточка radius 40 с тенью, иконка
 * Icon/Color/GoodWhite 48 и зелёное сообщение 16/18 Medium по центру,
 * крестик закрытия в правом верхнем углу. Канва не как у Modal: карточка
 * на любой ширине (макет мобайла 2329-148575 — карточка, не шит), фон
 * не затемняется — лента остаётся видна за попапом. Потребители: «Прочитать
 * все» («Все уведомления прочитаны») и «Удалить все» («Все уведомления
 * удалены») центра уведомлений.
 *
 * Цвет сообщения — Color/Green #00A63E из макета: канон успеха
 * --dl-success (#34C771) другой, не тронут до решения владельца (тот же
 * случай, что DESIGN.md §9 у макета 1877-68603).
 */
export type SuccessPopupProps = {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly message: string;
};

export function SuccessPopup({ open, onOpenChange, message }: SuccessPopupProps): JSX.Element {
  return (
    <DialogRoot open={open} onOpenChange={onOpenChange}>
      <DialogPortal>
        <DialogOverlay className="fixed inset-0 z-50" />
        <div className="fixed inset-0 z-50 flex items-center justify-center p-6">
          <DialogContent
            className={cn(
              'relative w-full max-w-[400px] rounded-sheet bg-white p-6 font-sans shadow-[0px_8px_24px_0px_rgba(0,0,0,0.12)] outline-none',
              'data-[state=open]:animate-[modal-card-in_400ms_var(--dl-ease)]',
              'data-[state=closed]:animate-[modal-card-out_220ms_var(--dl-ease)]',
            )}
          >
            <div className="flex flex-col items-center gap-2">
              <StatusIconGood className="h-12 w-12" />
              <DialogTitle className="text-center text-base font-medium leading-[18px] text-[#00a63e]">
                {message}
              </DialogTitle>
            </div>
            <DialogClose asChild>
              <IconButton
                icon={<Cancel />}
                label="Закрыть"
                className="absolute right-3.5 top-3.5"
              />
            </DialogClose>
          </DialogContent>
        </div>
      </DialogPortal>
    </DialogRoot>
  );
}

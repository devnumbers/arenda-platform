'use client';

import { useEffect, useState, type JSX, type ReactNode } from 'react';
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
  Dialog as DialogRoot,
} from '@radix-ui/react-dialog';
import { Drawer } from 'vaul';
import { Cancel } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { IconButton } from './icon-button';
import { SheetDragHandle } from './sheet-drag-handle';

/** Адаптивная модалка-шелл дизайн-слой (резолюция #449; переработка
 * 2026-08-26 по просьбе владельца — плавность «как у Apple» и настоящий
 * свайп шита). Один публичный API (Modal/ModalTrigger/ModalClose/
 * ModalContent), под каптом — ветвление по вьюпорту: ≥ tablet (768px) —
 * центрированная карточка на Radix Dialog; уже — нижний шит на vaul
 * (Drawer поверх Radix — стандартный Drawer стека shadcn/ui): настоящий
 * drag с инерцией, закрытие свайпом вниз, ручка Drag Handle (Figma
 * 847:11295). Тайминги — на единой кривой --dl-ease (решение 2026-08-26):
 * вход шита 450ms / карточки 400ms с мягким хвостом, выходы короче
 * (280/220ms), фон 350/250ms — по характеру системных модалок Apple
 * (HIG Motion: движение короткое, сглаженное на обоих концах).
 * Заголовок обязателен (a11y: контент ссылается на Title); стиль —
 * Mobile/Heading/H3/600 (20/24 SemiBold). Крестик закрытия (showClose) —
 * только в карточке, по Figma-компоненту Modal (936:33834, варианты
 * Popup/Sheet): в шите закрытие — свайп вниз или тап по оверлею. */

/** SSR-безопасное определение широкого вьюпорта: до гидратации — true
 * (карточка), после — факт; модалки открываются только по взаимодействию
 * пользователя, к тому времени ветвление уже скорректировано. */
function useIsDesktop(): boolean {
  const [isDesktop, setIsDesktop] = useState(true);
  useEffect(() => {
    const mq = window.matchMedia('(min-width: 768px)');
    const update = () => setIsDesktop(mq.matches);
    update();
    mq.addEventListener('change', update);
    return () => mq.removeEventListener('change', update);
  }, []);
  return isDesktop;
}

type RootProps = {
  readonly children: ReactNode;
  readonly open?: boolean;
  readonly defaultOpen?: boolean;
  readonly onOpenChange?: (open: boolean) => void;
};

export function Modal({ children, open, defaultOpen, onOpenChange }: RootProps): JSX.Element {
  const isDesktop = useIsDesktop();
  if (isDesktop) {
    return (
      <DialogRoot open={open} defaultOpen={defaultOpen} onOpenChange={onOpenChange}>
        {children}
      </DialogRoot>
    );
  }
  return (
    <Drawer.Root open={open} defaultOpen={defaultOpen} onOpenChange={onOpenChange} repositionInputs={false}>
      {children}
    </Drawer.Root>
  );
}

export function ModalTrigger(props: { children: ReactNode; className?: string; asChild?: boolean }): JSX.Element {
  const isDesktop = useIsDesktop();
  return isDesktop ? <DialogTrigger {...props} /> : <Drawer.Trigger {...props} />;
}

export function ModalClose(props: { children: ReactNode; className?: string; asChild?: boolean }): JSX.Element {
  const isDesktop = useIsDesktop();
  return isDesktop ? <DialogClose {...props} /> : <Drawer.Close {...props} />;
}

export type ModalContentProps = {
  readonly title: ReactNode;
  readonly description?: ReactNode;
  readonly children: ReactNode;
  readonly className?: string;
  /** Крестик закрытия в правом верхнем углу карточки (только попап). */
  readonly showClose?: boolean;
  /** Скрыть заголовок визуально (sr-only): a11y-имя диалога остаётся,
   * дизайн-макеты без заголовка не ломаются. */
  readonly titleSrOnly?: boolean;
};

export function ModalContent({
  title,
  description,
  children,
  className,
  showClose = false,
  titleSrOnly = false,
}: ModalContentProps): JSX.Element {
  const isDesktop = useIsDesktop();
  const titleBlock = titleSrOnly ? (
    <DialogTitle className="sr-only">{title}</DialogTitle>
  ) : (
    <div className="flex items-start justify-between gap-4">
      <DialogTitle className="text-xl font-semibold leading-6 text-content">{title}</DialogTitle>
      {isDesktop && showClose && (
        <DialogClose asChild>
          <IconButton icon={<Cancel />} label="Закрыть" />
        </DialogClose>
      )}
    </div>
  );
  const body = (
    <>
      <SheetDragHandle className="tablet:hidden" />
      <div className="flex flex-col gap-4 overflow-y-auto p-6 pb-[max(1.5rem,env(safe-area-inset-bottom))]">
        {titleBlock}
        {description !== undefined && (
          <DialogDescription className="text-sm text-content-secondary">{description}</DialogDescription>
        )}
        {children}
      </div>
    </>
  );

  if (isDesktop) {
    return (
      <DialogPortal>
        <DialogOverlay
          className={cn(
            'fixed inset-0 z-50 bg-overlay',
            'data-[state=open]:animate-[modal-overlay-in_350ms_var(--dl-ease)]',
            'data-[state=closed]:animate-[modal-overlay-out_250ms_var(--dl-ease)]',
          )}
        />
        <div className="fixed inset-0 z-50 flex items-center justify-center p-6">
          <DialogContent
            className={cn(
              'flex max-h-[92dvh] w-full max-w-[520px] flex-col rounded-sheet bg-white font-sans outline-none',
              'data-[state=open]:animate-[modal-card-in_400ms_var(--dl-ease)]',
              'data-[state=closed]:animate-[modal-card-out_220ms_var(--dl-ease)]',
              className,
            )}
          >
            {body}
          </DialogContent>
        </div>
      </DialogPortal>
    );
  }

  return (
    <Drawer.Portal>
      <Drawer.Overlay
        className={cn(
          'fixed inset-0 z-50 bg-overlay',
          'data-[state=open]:animate-[modal-overlay-in_350ms_var(--dl-ease)]',
          'data-[state=closed]:animate-[modal-overlay-out_250ms_var(--dl-ease)]',
        )}
      />
      <Drawer.Content
        className={cn(
          'fixed inset-x-0 bottom-0 z-50 mx-auto flex max-h-[92dvh] w-full flex-col rounded-t-sheet bg-white font-sans outline-none',
          // Вход/выход анимирует сам vaul (slideFromBottom 0.5s на той же
          // кривой cubic-bezier(0.32,0.72,0,1)) — скоординировано с drag.
          className,
        )}
      >
        {body}
      </Drawer.Content>
    </Drawer.Portal>
  );
}

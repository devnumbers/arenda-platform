"use client";

import { useEffect, useState, type JSX, type ReactNode } from "react";
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
  Dialog as DialogRoot,
} from "@radix-ui/react-dialog";
import { Drawer } from "vaul";
import { cn } from "@/lib/cn";
import { Cancel } from "./icons";
import { IconButton } from "./icon-button";
import { SheetDragHandle } from "./sheet-drag-handle";

/** Адаптивная модалка-шелл дизайн-слой — перенос из apps/frontend
 * (shared/ui/design/modal.tsx; резолюция #449, плавность «как у Apple»).
 * Один публичный API (Modal/ModalTrigger/ModalClose/ModalContent), под
 * каптом — ветвление по вьюпорту: ≥768px — центрированная карточка на Radix
 * Dialog; уже — нижний шит на vaul (Drawer поверх Radix — стандартный Drawer
 * стека shadcn/ui): настоящий drag с инерцией, закрытие свайпом вниз, ручка
 * Drag Handle (Figma 847:11295). Тайминги — на единой кривой --dl-ease:
 * вход шита 450ms / карточки 400ms с мягким хвостом, выходы короче
 * (280/220ms), фон 350/250ms — по характеру системных модалок Apple
 * (HIG Motion). Закрытие по фону/Esc, фокус-трап и лок скролла —
 * библиотечные (Radix/vaul). Заголовок обязателен (a11y: контент ссылается
 * на Title). Крестик закрытия (showClose) — только в карточке; в шите
 * закрытие — свайп вниз или тап по оверлею. Отличие от исходника: иконки
 * инлайн-SVG (канон лендинга), без @svgr. */

/** SSR-безопасное определение широкого вьюпорта для канвы модалок
 * (шит ↔ карточка, порог 768 — отдельная канва). До гидратации — true
 * (карточка); модалки открываются только по взаимодействию пользователя,
 * к тому времени ветвление уже скорректировано. */
export function useIsDesktop(): boolean {
  const [isDesktop, setIsDesktop] = useState(true);
  useEffect(() => {
    const mq = window.matchMedia("(min-width: 768px)");
    const update = () => setIsDesktop(mq.matches);
    update();
    mq.addEventListener("change", update);
    return () => mq.removeEventListener("change", update);
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
  /** Дополнение к классу заголовка: каноника — H3 20/24, макетам с серой
   * меткой 16 (шит сортировки #499) даёт text-base font-medium
   * text-content-tertiary. */
  readonly titleClassName?: string;
  readonly description?: ReactNode;
  /** Дополнение к классу описания: каноника — 14px, макетам с R/400 16
   * (диалог подтверждения #499) даёт text-base text-content-secondary. */
  readonly descriptionClassName?: string;
  readonly children: ReactNode;
  readonly className?: string;
  /** Крестик закрытия в правом верхнем углу карточки (только попап). */
  readonly showClose?: boolean;
  /** Скрыть заголовок визуально (sr-only): a11y-имя диалога остаётся,
   * дизайн-макеты без заголовка не ломаются. */
  readonly titleSrOnly?: boolean;
  /** Управляемый выход (карта #1010): Presence из @radix-ui/react-presence
   * на React 19.2.4 размонтирует карточку мгновенно — выходная анимация
   * data-state=closed не стартует (воспроизводится и во фронте). Потребитель
   * при закрытии держит open=true ещё 250мс с closing=true: классы выхода
   * применяются детерминированно, затем open=false снимает всё. На
   * vaul-шите не используется — свайп/выход анимирует сам vaul. */
  readonly closing?: boolean;
  /** Куда вернуть фокус после закрытия: Radix при unmount восстанавливает
   * «элемент до диалога» ПОСЛЕ пользовательского focus() (мак-мышь кнопку
   * не фокусирует — там это body). preventDefault + свой focus — канонный
   * хук onCloseAutoFocus. */
  readonly onCloseAutoFocus?: (event: Event) => void;
};

export function ModalContent({
  title,
  titleClassName,
  description,
  descriptionClassName,
  children,
  className,
  showClose = false,
  titleSrOnly = false,
  closing = false,
  onCloseAutoFocus,
}: ModalContentProps): JSX.Element {
  const isDesktop = useIsDesktop();
  const titleBlock = titleSrOnly ? (
    <DialogTitle className="sr-only">{title}</DialogTitle>
  ) : (
    <div className="flex items-start justify-between gap-4">
      <DialogTitle className={cn("text-xl font-semibold leading-6 text-content", titleClassName)}>{title}</DialogTitle>
      {isDesktop && showClose && (
        <DialogClose asChild>
          <IconButton icon={<Cancel />} label="Закрыть" />
        </DialogClose>
      )}
    </div>
  );
  const body = (
    <>
      {/* Канва модалок не привязана к ярусам хрома: карточка от 768
       * (useIsDesktop) — литерал, не токен tablet: (тот с 481). */}
      <SheetDragHandle className="min-[768px]:hidden" />
      <div className="flex flex-col gap-4 overflow-y-auto p-6 pb-[max(1.5rem,env(safe-area-inset-bottom))]">
        {titleBlock}
        {description !== undefined && (
          <DialogDescription className={cn("text-sm text-content-secondary", descriptionClassName)}>
            {description}
          </DialogDescription>
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
            "fixed inset-0 z-50 bg-overlay",
            // Вход/выход — классами по фазе closing, не по data-state:
            // см. комментарий к closing в ModalContentProps. forwards в
            // выходных — держит последний кадр (opacity 0) до размонтирования:
            // без него анимация, доиграв раньше таймера unmount, отдаёт
            // базовые стили и панель/фон вспыхивают на полную яркость.
            closing
              ? "animate-[modal-overlay-out_250ms_var(--dl-ease)_forwards]"
              : "animate-[modal-overlay-in_350ms_var(--dl-ease)]",
          )}
        />
        <div className="fixed inset-0 z-50 flex items-center justify-center p-6">
          <DialogContent
            {...(onCloseAutoFocus ? { onCloseAutoFocus } : {})}
            className={cn(
              // relative — канон попапа: крестик в углу карточки якорится в
              // карточку, а не в фуллскрин-обёртку. База — ПЕРВЫМ аргументом
              // слияния, className потребителя — последним (tailwind-merge
              // отдаёт приоритет последнему).
              "relative flex max-h-[92dvh] w-full max-w-[520px] flex-col rounded-sheet bg-white font-sans outline-none",
              closing
                ? "animate-[modal-card-out_220ms_var(--dl-ease)_forwards]"
                : "animate-[modal-card-in_400ms_var(--dl-ease)]",
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
          "fixed inset-0 z-50 bg-overlay",
          "data-[state=open]:animate-[modal-overlay-in_350ms_var(--dl-ease)]",
          "data-[state=closed]:animate-[modal-overlay-out_250ms_var(--dl-ease)]",
        )}
      />
      <Drawer.Content
        {...(onCloseAutoFocus ? { onCloseAutoFocus } : {})}
        className={cn(
          // Позиционные классы канвы (fixed) — последними в слиянии: любой
          // className потребителя, конфликтный по position, иначе через
          // tailwind-merge снимает fixed, шит выпадает из фикс-канвы в поток
          // за сгиб экрана и попап «пропадает» (#771).
          className,
          // Вход/выход анимирует сам vaul (slideFromBottom 0.5s на кривой
          // cubic-bezier(0.32,0.72,0,1)) — скоординировано с drag.
          "fixed inset-x-0 bottom-0 z-50 mx-auto flex max-h-[92dvh] w-full flex-col rounded-t-sheet bg-white font-sans outline-none",
        )}
      >
        {body}
      </Drawer.Content>
    </Drawer.Portal>
  );
}

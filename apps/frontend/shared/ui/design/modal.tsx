import type { JSX, ReactNode } from 'react';
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  Dialog as DialogRoot,
  DialogTitle,
  DialogTrigger,
} from '@radix-ui/react-dialog';
import { cn } from '@/shared/lib/cn';
import { SheetDragHandle } from './sheet-drag-handle';

/** Адаптивная модалка-шелл дизайн-слоя (резолюция #449): один Radix Dialog,
 * узкие экраны — нижний шит (radius 40 сверху, выезд снизу, drag-handle),
 * широкие (≥ tablet) — центрированная модалка (radius 40, проявление с
 * масштабом, без ручки шита). Заголовок обязателен (a11y: DialogContent
 * ссылается на DialogTitle). */

export const Modal = DialogRoot;
export const ModalTrigger = DialogTrigger;
export const ModalClose = DialogClose;

export type ModalContentProps = {
  readonly title: ReactNode;
  readonly description?: ReactNode;
  readonly children: ReactNode;
  readonly className?: string;
};

export function ModalContent({ title, description, children, className }: ModalContentProps): JSX.Element {
  return (
    <DialogPortal>
      <DialogOverlay
        className={cn(
          'fixed inset-0 z-50 bg-overlay',
          'data-[state=open]:animate-[modal-overlay-in_200ms_ease-out]',
          'data-[state=closed]:animate-[modal-overlay-out_200ms_ease-in]',
        )}
      />
      <div className="fixed inset-0 z-50 flex items-end justify-center tablet:items-center tablet:p-6">
        <DialogContent
          className={cn(
            'flex max-h-[92dvh] w-full flex-col rounded-t-sheet bg-white font-sans outline-none',
            'tablet:max-w-[560px] tablet:rounded-sheet',
            'data-[state=open]:animate-[modal-sheet-in_300ms_ease-out] tablet:data-[state=open]:animate-[modal-card-in_200ms_ease-out]',
            'data-[state=closed]:animate-[modal-sheet-out_200ms_ease-in] tablet:data-[state=closed]:animate-[modal-card-out_200ms_ease-in]',
            className,
          )}
        >
          <SheetDragHandle className="tablet:hidden" />
          <div className="flex flex-col gap-4 overflow-y-auto p-6 pb-[max(1.5rem,env(safe-area-inset-bottom))]">
            <DialogTitle className="text-lg font-semibold text-content">{title}</DialogTitle>
            {description !== undefined && (
              <DialogDescription className="text-sm text-content-secondary">{description}</DialogDescription>
            )}
            {children}
          </div>
        </DialogContent>
      </div>
    </DialogPortal>
  );
}

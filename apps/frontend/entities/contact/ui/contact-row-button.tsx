'use client';

import type { JSX } from 'react';
import { BoldUser } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { useKeyboardActivation } from '@/shared/lib/hooks/useKeyboardActivation';
import { contactFullName } from '../lib/full-name';
import type { Contact } from '../model/types';

/** Поверхность, на которой лежит строка: аватар инвертируется относительно
 * неё (макет 1527:74139 — белый круг на серой карточке; 1527:74837 — серый
 * круг на белом фоне результатов поиска). */
export type ContactRowSurface = 'muted' | 'white';

export type ContactRowButtonProps = {
  readonly contact: Contact;
  readonly surface?: ContactRowSurface;
  /** Основное действие строки; без него строка статична (карточка контакта —
   * #510, в #508 строки не кликабельны). */
  readonly onSelect?: () => void;
  readonly className?: string;
};

/**
 * Строка контакта (компонент Figma «Row Button», 936:39348): аватар-круг 44
 * с BoldUser, заголовок — полное имя, подзаголовок — роль; телефона в строке
 * нет (макет 1527:74139, решение владельца). Hover/press приглушают строку.
 * Каноническая строка списков контактов (DESIGN.md, «Строки списков»):
 * живёт в срезе сущности — нужна и книге объекта (#508), и шагу «Контакт
 * арендатора» визарда аренды (#530).
 */
export function ContactRowButton({
  contact,
  surface = 'muted',
  onSelect,
  className,
}: ContactRowButtonProps): JSX.Element {
  const activatorProps = useKeyboardActivation({ onSelect });

  return (
    <div
      {...activatorProps}
      className={cn(
        'group/row flex w-full items-center gap-2 py-2 text-left outline-none',
        'transition-opacity focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        'hover:opacity-80 active:opacity-80',
        onSelect === undefined ? 'cursor-default' : 'cursor-pointer',
        className,
      )}
    >
      <span
        aria-hidden
        className={cn(
          'flex h-11 w-11 shrink-0 items-center justify-center rounded-pill',
          surface === 'muted'
            ? 'bg-surface shadow-[0_0_0_2.5px_var(--dl-surface-muted)]'
            : 'bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]',
        )}
      >
        <BoldUser className="h-6 w-6" />
      </span>
      <span className="flex min-w-0 flex-1 flex-col justify-center gap-1">
        <span className="truncate text-base font-medium text-content">{contactFullName(contact)}</span>
        {contact.role.length > 0 && (
          <span className="truncate text-sm text-content-secondary">{contact.role}</span>
        )}
      </span>
    </div>
  );
}

'use client';

import type { JSX } from 'react';
import { UserAvatar } from '@/shared/ui/design';
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
  /** Подзаголовок вместо роли — плоская книга пишет «Роль (Объект)»
   * (макеты 1726:65083/85937); по умолчанию роль. */
  readonly subtitle?: string;
  /** Основное действие строки; без него строка статична (карточка контакта —
   * #510, в #508 строки не кликабельны). */
  readonly onSelect?: () => void;
  readonly className?: string;
};

/**
 * Строка контакта (компонент Figma «Row Button», 936:39348): круг 44
 * с BoldUser либо фото карточки (тикет #1229 — фото стримится с
 * `private, max-age=300`, строки книг перечитываются после фото-мутаций,
 * бастер здесь не нужен), заголовок — полное имя, подзаголовок — роль;
 * телефона в строке
 * нет (макет 1527:74139, решение владельца). Hover/press приглушают строку.
 * Каноническая строка списков контактов (DESIGN.md, «Строки списков»):
 * живёт в срезе сущности — нужна и книге объекта (#508), и шагу «Контакт
 * арендатора» визарда аренды (#530).
 */
export function ContactRowButton({
  contact,
  surface = 'muted',
  subtitle,
  onSelect,
  className,
}: ContactRowButtonProps): JSX.Element {
  const activatorProps = useKeyboardActivation({ onSelect });
  const rowSubtitle = subtitle ?? (contact.role.length > 0 ? contact.role : undefined);

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
      <UserAvatar variant={surface} photoUrl={contact.photoUrl} />
      <span className="flex min-w-0 flex-1 flex-col justify-center gap-1">
        <span className="truncate text-base font-medium text-content">{contactFullName(contact)}</span>
        {rowSubtitle !== undefined && (
          <span className="truncate text-sm text-content-secondary">{rowSubtitle}</span>
        )}
      </span>
    </div>
  );
}

'use client';

import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import type { ParticipantAccessRole } from '@/entities/participants';

/** Сегмент-переключатель роли доступа (карта #692, тикет #698; макеты
 * 2177-59620 «Права участника» и 2010-131724 шит «Пригласить в объект»):
 * серая капсула radius 16 с паддингом 2, активный сегмент — белая пилюля
 * radius 14 с тенью, подписи 14/500 (решение чарта: роли только
 * отображение — full_access «Редактирование», viewer «Просмотр»).
 * Потребителей двое в этом же виджете; третий — кандидат на вынос в
 * design-слой. */
export function ParticipantRoleSegmented({
  value,
  disabled = false,
  onChange,
  ariaLabel = 'Роль доступа',
}: {
  readonly value: ParticipantAccessRole;
  readonly disabled?: boolean;
  readonly onChange: (role: ParticipantAccessRole) => void;
  readonly ariaLabel?: string;
}): JSX.Element {
  const options: ReadonlyArray<{
    readonly role: ParticipantAccessRole;
    readonly label: string;
  }> = [
    { role: 'viewer', label: 'Просмотр' },
    { role: 'full_access', label: 'Редактирование' },
  ];

  return (
    <div
      role="radiogroup"
      aria-label={ariaLabel}
      className="flex rounded-2xl bg-surface-muted p-0.5"
    >
      {options.map((option) => {
        const selected = value === option.role;
        return (
          <button
            key={option.role}
            type="button"
            role="radio"
            aria-checked={selected}
            disabled={disabled}
            onClick={() => onChange(option.role)}
            className={cn(
              'h-12 flex-1 rounded-[14px] text-sm font-medium outline-none transition-colors',
              'focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface',
              disabled && 'cursor-default opacity-60',
              selected
                ? 'bg-surface text-content shadow-[0_2px_8px_rgba(0,0,0,0.16)]'
                : 'text-content-secondary',
            )}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}

'use client';

import type { ComponentType, JSX } from 'react';
import Link from 'next/link';
import { AddSmall, CheckSmall, EditSmall, TrashBinSmall } from '@/shared/assets/icons';
import {
  baseActionLabel,
  baseActionTone,
  historySegmentHref,
  type HistoryBaseActionTone,
  type HistoryEntry,
} from '@/entities/history';
import { formatTime } from '@/shared/lib/date-format';
import { cn } from '@/shared/lib/cn';

/** Иконка основного действия по канону иконок: S-стиль 16×16 (макет
 * 2157-56876) — нейтральный тёмный глиф, тон строки несёт полоска слева. */
const TONE_ICON: Record<HistoryBaseActionTone, ComponentType<{ className?: string }>> = {
  success: AddSmall,
  warning: EditSmall,
  primary: CheckSmall,
  danger: TrashBinSmall,
};

/** Полоска тона — палитра макета 2157-56876 (решение владельца 23.09:
 * литералы макета). Синий и красный совпадают с токенами primary/danger,
 * зелёный/янтарный макета темнее токенов success/warning. */
const TONE_BAR: Record<HistoryBaseActionTone, string> = {
  success: 'bg-[#00a63e]',
  warning: 'bg-[#ffb900]',
  primary: 'bg-primary',
  danger: 'bg-[#fb2c36]',
};

/**
 * Строка ленты «История действий» (#709, макет 2157-56876): цветная
 * полоска тона 3px на всю высоту строки, S-иконка основного действия,
 * текст — серверные сегменты дословно (ADR 0061 §6); связанные фрагменты —
 * синие с подчёркиванием, переход по ссылке сегмента на страницу сущности
 * (#713): резолвер — entities/history, строка не кнопка, переходом служат
 * только синие фрагменты. Время — справа по нижней строке текста
 * (items-end).
 */
export function HistoryRow({ entry }: { readonly entry: HistoryEntry }): JSX.Element {
  const tone = baseActionTone(entry.baseAction);
  const Icon = TONE_ICON[tone];
  return (
    <div className="flex items-end gap-2">
      <div className="flex min-w-0 flex-1 items-start gap-2">
        <span aria-hidden className={cn('w-[3px] shrink-0 self-stretch rounded-pill', TONE_BAR[tone])} />
        <Icon
          className="h-4 w-4 shrink-0 text-content"
          aria-label={baseActionLabel(entry.baseAction)}
        />
        <p className="min-w-0 flex-1 text-xs leading-[15px] text-content">
          {entry.segments.map((segment, index) => {
            const href = segment.link ? historySegmentHref(segment.link, entry.propertyId) : null;
            return href ? (
              <Link
                key={index}
                href={href}
                className="text-primary underline decoration-from-font"
              >
                {segment.text}
              </Link>
            ) : (
              <span key={index}>{segment.text}</span>
            );
          })}
        </p>
      </div>
      <span className="shrink-0 text-xs leading-[15px] text-content-secondary">
        {formatTime(entry.createdAt)}
      </span>
    </div>
  );
}

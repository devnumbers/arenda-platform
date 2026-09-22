'use client';

import type { ComponentType, JSX } from 'react';
import { Add, Check, Edit, TrashBin } from '@/shared/assets/icons';
import {
  baseActionLabel,
  baseActionTone,
  type HistoryBaseActionTone,
  type HistoryEntry,
} from '@/entities/history';
import { formatTime } from '@/shared/lib/date-format';
import { cn } from '@/shared/lib/cn';

/** Иконка основного действия по канону иконок (цвет — текстовый класс
 * тона; glyph — штатные Add/Edit/Check/TrashBin, макет 2157-56786). */
const TONE_ICON: Record<HistoryBaseActionTone, ComponentType<{ className?: string }>> = {
  success: Add,
  warning: Edit,
  primary: Check,
  danger: TrashBin,
};

const TONE_TEXT: Record<HistoryBaseActionTone, string> = {
  success: 'text-success',
  warning: 'text-warning',
  primary: 'text-primary',
  danger: 'text-danger',
};

/**
 * Строка ленты «История действий» (#709, макет 2157-56786): цветная
 * иконка основного действия, текст строки — серверные сегменты дословно
 * (ADR 0061 §6), связанные фрагменты — синим (переходы по ссылкам
 * сегментов — тикет #713), время справа. Строка не кнопка: переходом
 * служат только синие фрагменты.
 */
export function HistoryRow({ entry }: { readonly entry: HistoryEntry }): JSX.Element {
  const tone = baseActionTone(entry.baseAction);
  const Icon = TONE_ICON[tone];
  return (
    <div className="flex items-start gap-3 py-[5px]">
      <Icon
        className={cn('mt-0.5 h-6 w-6 shrink-0', TONE_TEXT[tone])}
        aria-label={baseActionLabel(entry.baseAction)}
      />
      <p className="min-w-0 flex-1 text-[15px] leading-5 text-content">
        {entry.segments.map((segment, index) =>
          segment.link ? (
            <span key={index} className="text-primary">
              {segment.text}
            </span>
          ) : (
            <span key={index}>{segment.text}</span>
          ),
        )}
      </p>
      <span className="ml-2 shrink-0 pt-px text-sm leading-4 text-content-tertiary">
        {formatTime(entry.createdAt)}
      </span>
    </div>
  );
}

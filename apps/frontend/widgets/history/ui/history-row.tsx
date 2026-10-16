import type { ComponentType, JSX, SVGProps } from 'react';
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
import styles from './history-row.module.css';

/** Иконка основного действия по канону иконок: S-стиль 16×16 (макет
 * 2157-56876) — нейтральный тёмный глиф, тон строки несёт полоска слева. */
const TONE_ICON: Record<HistoryBaseActionTone, ComponentType<SVGProps<SVGSVGElement>>> = {
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
 * Строка ленты «История действий» (#709; новый шаблон — карта #1324, D1,
 * макет 2157-55133): цветная полоска тона 3px на всю высоту строки,
 * S-иконка основного действия, заголовок — серверные сегменты дословно
 * (ADR 0061 §6), связанные фрагменты — синие с подчёркиванием, переход по
 * ссылке сегмента на страницу сущности (#713): резолвер — entities/history,
 * строка не кнопка, переходом служат только синие фрагменты. Под заголовком
 * — серые (#6F787C) строки-пояснения details, собранные сервером при записи
 * и отрендеренные дословно; у строк без пояснений (записанные до D1 и
 * действия без диффов) — один заголовок. Длинные фрагменты заголовка и
 * пояснений переносятся, многострочные значения (описание объекта) читаются
 * как набрано (карта #1444). Время — справа по нижней строке
 * текста (items-end).
 *
 * Свежая строка live-влития (#880, механика F демо #879): мягкое
 * появление row-in (350мс, --dl-duration-move) и метка «новое» —
 * анимация уже созданной строки, не свап значения; метку гасит
 * экран ленты, когда читатель увидел строки (acknowledgeFresh в
 * history-feed-screen).
 */
export function HistoryRow({ entry, fresh = false }: { readonly entry: HistoryEntry; readonly fresh?: boolean }): JSX.Element {
  const tone = baseActionTone(entry.baseAction);
  const Icon = TONE_ICON[tone];
  return (
    <div className={cn('flex items-end gap-2', fresh && styles.rowIn)}>
      <div className="flex min-w-0 flex-1 items-start gap-2">
        <span aria-hidden className={cn('w-[3px] shrink-0 self-stretch rounded-pill', TONE_BAR[tone])} />
        <Icon
          className="h-4 w-4 shrink-0 text-content"
          role="img"
          aria-label={baseActionLabel(entry.baseAction)}
        />
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          {/* Длинные фрагменты переносятся, а не растягивают строку ленты
           *  (бриф владельца 15.10, карта #1444); канон переноса —
           *  overflow-wrap: break-word, прецедент PropertyInfoCard
           *  (коммит 6ddbca31b). Пояснения — pre-wrap: многострочное
           *  описание читается как набрано. */}
          <p className="break-words text-xs leading-[15px] text-content">
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
            {fresh && (
              /* Метка «новое» (макет-механика F демо #879): чип 10/600
               * на --dl-surface-info рядом с текстом строки. */
              <span className="ml-1.5 inline-block rounded-md bg-surface-info px-1.5 py-0.5 align-baseline text-[10px] font-semibold leading-[13px] text-primary">
                новое
              </span>
            )}
          </p>
          {entry.details?.map((line, index) => (
            <p key={index} className="whitespace-pre-wrap break-words text-xs leading-[15px] text-content-secondary">
              {line}
            </p>
          ))}
        </div>
      </div>
      <span className="shrink-0 text-xs leading-[15px] text-content-secondary">
        {formatTime(entry.createdAt)}
      </span>
    </div>
  );
}

'use client';

import type { JSX } from 'react';

import type { InvitePropertyOption, InviteSelectionState } from '@/entities/participants';
import { ObjectAvatarGlyph, PARTICIPANT_ROW_BASE_CLASS, SelectionGlyph } from './participant-fragments';

/** Ряды выбора объектов приглашения — общая разметка мультичека
 * («Все объекты» + ряд на объект, макеты 2008-46627 / 2177-59620):
 * приглашение хаба (#699, пикер в диалоге выбора) и «Пригласить в
 * объект» (#698, список на странице) различаются только обёрткой
 * отступов и шириной разделителя. Состояние ряда «Все объекты»
 * считается каноном inviteSelectionState на стороне потребителя. */
export function InvitePropertiesRows({
  options,
  selected,
  allState,
  onToggleAll,
  onToggle,
  dividerClassName = 'h-px bg-surface-muted',
}: {
  readonly options: ReadonlyArray<InvitePropertyOption>;
  readonly selected: ReadonlySet<string>;
  readonly allState: InviteSelectionState;
  readonly onToggleAll: () => void;
  readonly onToggle: (propertyId: string) => void;
  readonly dividerClassName?: string;
}): JSX.Element {
  // Канон tri-state ('all'/'partial'/'none') → пиктограмма выбора
  // ('on'/'mixed'/'off') и значение aria-checked.
  const allGlyphState = allState === 'all' ? 'on' : allState === 'partial' ? 'mixed' : 'off';
  return (
    <>
      <button
        type="button"
        role="checkbox"
        aria-checked={allState === 'partial' ? 'mixed' : allState === 'all'}
        onClick={onToggleAll}
        className={PARTICIPANT_ROW_BASE_CLASS}
      >
        <ObjectAvatarGlyph photoUrl={undefined} isAll />
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="truncate text-base font-medium leading-[18px] text-content">
            Все объекты
          </span>
          <span className="truncate text-sm leading-4 text-content-secondary">
            Поделиться всеми объектами
          </span>
        </span>
        <SelectionGlyph state={allGlyphState} />
      </button>
      <div aria-hidden className={dividerClassName} />
      {options.map((option) => (
        <button
          key={option.id}
          type="button"
          role="checkbox"
          aria-checked={selected.has(option.id)}
          onClick={() => onToggle(option.id)}
          className={PARTICIPANT_ROW_BASE_CLASS}
        >
          <ObjectAvatarGlyph photoUrl={option.photoUrl} type={option.type} />
          <span className="flex min-w-0 flex-1 flex-col gap-1">
            <span className="truncate text-base font-medium leading-[18px] text-content">
              {option.name}
            </span>
            <span className="truncate text-sm leading-4 text-content-secondary">
              {option.address}
            </span>
          </span>
          <SelectionGlyph state={selected.has(option.id) ? 'on' : 'off'} />
        </button>
      ))}
    </>
  );
}

'use client';

import type { JSX, ReactNode } from 'react';
import { Check } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { Menu, MenuContent, MenuItem, MenuTrigger } from './menu';
import { Modal, ModalContent, ModalTrigger, useIsDesktop } from './modal';

/** Адаптивный пикер опций (общий, из сортировок задач #499 и контактов
 * #508): триггер-кнопка (children) открывает выбор одним значением —
 * на десктопе карточку-меню 246px с пунктами и selection-квадратами
 * (Figma 1603-94487), на мобиле — нижний шит с рядами и radio-кружками
 * (Figma 1535-76225, 1539:85395). Опции сгруппированы, между группами —
 * линия. Семантика — «радио»: тап применяет выбор сразу; меню закрывается
 * (Radix), шит остаётся до закрытия свайпом/оверлеем (решение владельца
 * 2026-09-03). Триггер — готовая кнопка: обработчики открытия передаются
 * ей через asChild, поэтому она должна прокидывать пропсы кнопки
 * (…props на свою <button>). Открытое состояние держат Radix и vaul —
 * компоненту состояние не нужно.
 *
 * Спек меню-пикера — прежний (макет 1603-94487): карточка py 12px и
 * тень 0 8px 24px, пункты 44px/px 20. Новый кебаб-спек 3183-86541
 * (menu.tsx) на сортировки не переводили (решение владельца 22.10) —
 * пин-константы ниже побеждают новую базу через cn; снимаются одной
 * правкой, когда макет пикера обновят. */
export const pickerMenuContentPinClasses =
  'py-3 shadow-[0_8px_24px_rgba(23,26,28,0.12)]';

export const pickerMenuPinnedRowClasses = 'h-11 min-h-11 px-5';
export type PickerMenuOption = {
  readonly label: string;
  readonly selected: boolean;
  readonly onSelect: () => void;
};

export type PickerMenuGroup = {
  readonly options: ReadonlyArray<PickerMenuOption>;
};

export type PickerMenuProps = {
  /** Заголовок мобильного шита (в меню на десктопе не показывается). */
  readonly title: string;
  readonly groups: ReadonlyArray<PickerMenuGroup>;
  readonly children: ReactNode;
};

export function PickerMenu({ title, groups, children }: PickerMenuProps): JSX.Element {
  const isDesktop = useIsDesktop();

  if (isDesktop) {
    return (
      <Menu>
        <MenuTrigger asChild>{children}</MenuTrigger>
        <MenuContent
          align="start"
          className={cn('w-[246px] gap-0', pickerMenuContentPinClasses)}
        >
          {groups.map((group, index) => (
            <div
              key={index}
              className={cn(
                'flex flex-col gap-0.5',
                index > 0 && 'mt-3 border-t border-surface-muted pt-3',
              )}
            >
              {group.options.map((option) => (
                <PickerMenuRow key={option.label} variant="menu" option={option} />
              ))}
            </div>
          ))}
        </MenuContent>
      </Menu>
    );
  }

  return (
    <Modal>
      <ModalTrigger asChild>{children}</ModalTrigger>
      <ModalContent
        title={title}
        titleClassName="text-base font-medium leading-[18px] text-content-tertiary"
      >
        <div className="flex flex-col gap-6" role="radiogroup" aria-label={title}>
          {groups.map((group, index) => (
            <div
              key={index}
              className={cn(
                'flex flex-col gap-2',
                index > 0 && 'border-t border-surface-muted pt-6',
              )}
            >
              {group.options.map((option) => (
                <PickerMenuRow key={option.label} variant="sheet" option={option} />
              ))}
            </div>
          ))}
        </div>
      </ModalContent>
    </Modal>
  );
}

type PickerMenuRowProps = {
  readonly variant: 'menu' | 'sheet';
  readonly option: PickerMenuOption;
};

/** Опция: в меню — пункт с ведущим selection-квадратом (Figma 1186:44732 +
 * Selection Button Checkbox, radius 8: синий с белой галкой у выбранного,
 * серое кольцо у остальных); в шите — серая плашка 48 radius 16 с
 * radio-кружком справа (Figma Row Button 1041:48796). Шаги hover/active —
 * muted-пара серой плашки (радио-диалект цветовых шагов, без скейла). */
function PickerMenuRow({ variant, option }: PickerMenuRowProps): JSX.Element {
  if (variant === 'menu') {
    return (
      <MenuItem
        className={pickerMenuPinnedRowClasses}
        onSelect={option.onSelect}
        icon={
          option.selected ? (
            <span className="flex h-5 w-5 items-center justify-center rounded-lg bg-primary text-white">
              <Check className="h-4 w-4" aria-hidden />
            </span>
          ) : (
            <span className="h-5 w-5 rounded-lg border-[1.5px] border-content-tertiary" aria-hidden />
          )
        }
      >
        {option.label}
      </MenuItem>
    );
  }
  return (
    <button
      type="button"
      role="radio"
      aria-checked={option.selected}
      onClick={option.onSelect}
      className={cn(
        'flex h-12 w-full cursor-pointer items-center justify-between gap-3 rounded-button bg-surface-muted pr-3 pl-5 text-left font-sans outline-none transition-colors',
        'hover:bg-surface-muted-hover active:bg-surface-muted-active',
        'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface',
      )}
    >
      <span className="text-base font-medium text-content">{option.label}</span>
      {option.selected ? (
        <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-pill bg-primary text-white">
          <Check className="h-4 w-4" aria-hidden />
        </span>
      ) : (
        <span className="h-6 w-6 shrink-0 rounded-pill border-[1.5px] border-content-tertiary" aria-hidden />
      )}
    </button>
  );
}

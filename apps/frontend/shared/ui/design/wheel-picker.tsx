'use client';

import {
  useEffect,
  useId,
  useRef,
  useState,
  type JSX,
  type KeyboardEvent,
  type MouseEvent,
} from 'react';
import { cn } from '@/shared/lib/cn';

/** Колесо-«крутилка» дизайн-слоя (редизайн 2026-09-04 по решению
 * владельца, Figma 1539-82659): вертикальный список с нативным скроллом
 * и снапом к ряду, инерция тач-скролла, ленты-градиенты сверху и снизу и
 * серая полоса выбора за центральной строкой. Геометрия: окно 240px
 * (класс h-[240px]) = 5 рядов по 48px (WHEEL_ROW_HEIGHT) — по 2 ряда над
 * и под выбранным (правка владельца: видно на 2 ряда, не 3), поля по
 * 96px (класс py-[96px]) — ряд садится в полосу пиксель в пиксель.
 * Полоса выбора 48px без закруглений (правка владельца — прямые углы).
 * Кегль всех рядов единый — Mobile/Text/XL (20/24 Regular), различие
 * только цветом: выбранный #171A1C (content), остальные #6F787C
 * (content-secondary) — каскад кеглей старого макета 848:8720 отменён.
 * Значение коммитится,
 * когда прокрутка осела; клик по ряду и клавиатура коммитят сразу.
 *
 * Клавиатура — собственная, а не общий модуль listbox-keyboard: то колесо
 * со скролл-снапом и aria-activedescendant, без roving focus, без Enter/
 * Space/Escape и без заворота по краям — семантика стрелок иная. */

/** Высота ряда (px); окно колеса — 7 рядов. */
const WHEEL_ROW_HEIGHT = 48;
/** Пауза тишины скролла, после которой ряд под полосой считается выбранным. */
const SETTLE_TIMEOUT_MS = 150;

export type WheelPickerItem = {
  readonly value: string;
  readonly label: string;
};

export type WheelPickerProps = {
  readonly items: ReadonlyArray<WheelPickerItem>;
  readonly value: string;
  readonly onValueChange: (value: string) => void;
  /** Имя колеса для скринридеров («Месяц», «Год»). */
  readonly label: string;
  /** Прокрутка дошла до края списка (за 2 ряда) — родитель может удлинить
   * items (бесконечная лента годов пикера месяц/год). */
  readonly onNearEnd?: () => void;
  /** Полоса выбора за центральным рядом; false — когда полосу рисует общий
   * контейнер (WheelPickerSheet: одна полоса на все колонки, скругление
   * только по внешним краям — решение владельца 2026-09-04). */
  readonly strip?: boolean;
  readonly className?: string;
};

export function WheelPicker({
  items,
  value,
  onValueChange,
  label,
  onNearEnd,
  strip = true,
  className,
}: WheelPickerProps): JSX.Element {
  const listRef = useRef<HTMLUListElement | null>(null);
  const settleTimerRef = useRef<number | null>(null);
  // Первая установка колеса — мгновенная (как у iOS: выбранная строка уже
  // в центре при открытии), дальнейшие синхронизации — плавные.
  const instantRef = useRef(true);
  const optionId = useId();
  const valueIndex = Math.max(items.findIndex((item) => item.value === value), 0);
  const [scrollIndex, setScrollIndex] = useState(valueIndex);

  useEffect(
    () => () => {
      if (settleTimerRef.current !== null) window.clearTimeout(settleTimerRef.current);
    },
    [],
  );

  // Колесо следует за значением: внешний выбор или клавиатура плавно
  // докручивают нужный ряд в центр; после собственного скролла человека
  // ничего не дёргается — позиция уже совпадает с целевой. Актуальный
  // индекс для типографики приходит из событий скролла этого прокрута.
  useEffect(() => {
    const list = listRef.current;
    if (list !== null && Math.round(list.scrollTop / WHEEL_ROW_HEIGHT) !== valueIndex) {
      list.scrollTo({
        top: valueIndex * WHEEL_ROW_HEIGHT,
        behavior: instantRef.current ? 'auto' : 'smooth',
      });
    }
    instantRef.current = false;
  }, [valueIndex]);

  const commitIndex = (index: number): void => {
    const item = items[index];
    if (item !== undefined && item.value !== value) {
      onValueChange(item.value);
    }
  };

  const handleScroll = (): void => {
    const list = listRef.current;
    if (list === null) return;
    const maxIndex = items.length - 1;
    const index = Math.max(
      0,
      Math.min(Math.round(list.scrollTop / WHEEL_ROW_HEIGHT), maxIndex),
    );
    setScrollIndex(index);
    if (onNearEnd !== undefined && index >= maxIndex - 1) {
      onNearEnd();
    }
    if (settleTimerRef.current !== null) window.clearTimeout(settleTimerRef.current);
    settleTimerRef.current = window.setTimeout(() => commitIndex(index), SETTLE_TIMEOUT_MS);
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLUListElement>): void => {
    let next: number;
    if (event.key === 'ArrowUp') next = Math.max(valueIndex - 1, 0);
    else if (event.key === 'ArrowDown') next = Math.min(valueIndex + 1, items.length - 1);
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = items.length - 1;
    else return;
    event.preventDefault();
    commitIndex(next);
  };

  // Клик по ряду — pointer-сокращение; клавиатурный эквивалент живёт на
  // самом списке (стрелки/Tab), поэтому клики делегированы контейнеру.
  const handleClick = (event: MouseEvent<HTMLUListElement>): void => {
    const option =
      event.target instanceof Element ? event.target.closest<HTMLElement>('li[data-index]') : null;
    const index = option?.dataset.index;
    if (index !== undefined) {
      commitIndex(Number(index));
    }
  };

  return (
    <div className={cn('relative', className)}>
      {strip && (
        <div
          aria-hidden
          className="pointer-events-none absolute inset-x-0 top-1/2 h-12 -translate-y-1/2 bg-surface-muted"
        />
      )}
      <ul
        ref={listRef}
        role="listbox"
        aria-label={label}
        aria-activedescendant={`${optionId}-${valueIndex}`}
        tabIndex={0}
        onScroll={handleScroll}
        onKeyDown={handleKeyDown}
        onClick={handleClick}
        className="relative flex h-[240px] list-none flex-col snap-y snap-mandatory overflow-y-scroll overscroll-y-contain py-[96px] outline-none [scrollbar-width:none] focus-visible:ring-2 focus-visible:ring-primary [&::-webkit-scrollbar]:hidden"
      >
        {items.map((item, index) => {
          const distance = Math.abs(index - scrollIndex);
          return (
            <li
              key={item.value}
              id={`${optionId}-${index}`}
              role="option"
              aria-selected={index === valueIndex}
              data-index={index}
              style={{ height: WHEEL_ROW_HEIGHT }}
              className={cn(
                // Единый кегль всех рядов (Figma 1539-82659): выбранный
                // темнеет, остальные серые — без каскада размеров.
                'flex shrink-0 cursor-pointer snap-center items-center justify-center font-sans text-xl leading-6 text-content-secondary transition-colors',
                distance === 0 && 'text-content',
              )}
            >
              {item.label}
            </li>
          );
        })}
      </ul>
      <div
        aria-hidden
        className="pointer-events-none absolute inset-x-0 top-0 h-12 bg-gradient-to-b from-surface to-transparent"
      />
      <div
        aria-hidden
        className="pointer-events-none absolute inset-x-0 bottom-0 h-12 bg-gradient-to-t from-surface to-transparent"
      />
    </div>
  );
}

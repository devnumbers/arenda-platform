"use client";

import type { JSX } from "react";
import clsx from "clsx";
import { SmallArrowDown } from "@/shared/assets/icons";
import { ChipButton } from "@/shared/ui/design";

export type OperationsFilterChipsProps = {
  /** Лейбл чипа периода: дефолт — «Сентябрь 2026», выбор — «1 — 30 ноя»,
   * глобальная лента без периода — нейтральный «Период» (#670). */
  readonly periodLabel: string;
  /** Период применён — чип синий; по умолчанию true: у непереведённых на
   * «весь период» экранов период применён всегда (#670). */
  readonly periodActive?: boolean;
  /** Лейбл чипа категорий: «Все категории», имя или счётчик. */
  readonly categoriesLabel: string;
  /** Активный фильтр категорий подсвечивает чип синим, как период. */
  readonly categoriesActive: boolean;
  readonly onOpenPeriod: () => void;
  readonly onOpenCategories: () => void;
  /** Чип «Объект» глобальной ленты (#541): между периодом и категорией,
   * как в макете 1733-26973. Без props чипа нет — объектные экраны (#474)
   * объект не фильтруют. */
  readonly propertyLabel?: string;
  /** Активный мультивыбор подсвечивает чип синим (макет 1733-26973). */
  readonly propertyActive?: boolean;
  readonly onOpenProperties?: () => void;
  /** Боковые вставки контейнера: объектные экраны (#474, PageContent без
   * горизонтального паддинга) передают px-6; глобальная лента (#541) живёт
   * в контенте кабинета с его собственным паддингом — ей вставка не нужна
   * (правило владельца: сервисные элементы по краю контента страницы,
   * без своей вставки). */
  readonly className?: string;
};

/**
 * Строка чипов фильтров экранов операций (#474/#477): «Период» синий, пока
 * период применён (periodActive; у объектных экранов — всегда, у глобальной
 * ленты #670 — только с явным диапазоном, в дефолте серый «Период»),
 * «Категория» — синий только с активным фильтром. Чипы открывают свои шиты
 * выбора (#477, Figma 1492-41825). Глобальная лента (#541) добавляет между
 * ними чип «Объект» — лейбл «Все объекты»/«1 объект»/«N объектов», синий
 * с активным фильтром.
 */
export function OperationsFilterChips({
  periodLabel,
  periodActive = true,
  categoriesLabel,
  categoriesActive,
  onOpenPeriod,
  onOpenCategories,
  propertyLabel,
  propertyActive = false,
  onOpenProperties,
  className,
}: OperationsFilterChipsProps): JSX.Element {
  return (
    <div className={clsx("flex gap-1.5 overflow-x-auto", className)}>
      <ChipButton
        selected={periodActive}
        trailingIcon={<SmallArrowDown />}
        onClick={onOpenPeriod}
      >
        {periodLabel}
      </ChipButton>
      {propertyLabel !== undefined && onOpenProperties !== undefined && (
        <ChipButton
          selected={propertyActive}
          trailingIcon={<SmallArrowDown />}
          onClick={onOpenProperties}
        >
          {propertyLabel}
        </ChipButton>
      )}
      <ChipButton
        selected={categoriesActive}
        trailingIcon={<SmallArrowDown />}
        onClick={onOpenCategories}
      >
        {categoriesLabel}
      </ChipButton>
    </div>
  );
}

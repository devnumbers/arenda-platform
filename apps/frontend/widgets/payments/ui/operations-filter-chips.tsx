"use client";

import type { JSX } from "react";
import clsx from "clsx";
import { SmallArrowDown } from "@/shared/assets/icons";
import { ChipButton } from "@/shared/ui/design";
import {
  operationsPeriodChipLabel,
  type OperationsPeriod,
} from "@/features/payments";

export type OperationsFilterChipsProps = {
  /** Лейбл чипа периода: применённый диапазон — «1 — 30 ноя», без
   * применённого периода — нейтральный «Период» (#670/#674/#675). */
  readonly periodLabel: string;
  /** Период применён — чип синий; в дефолте «весь период» (карта #669)
   * все экраны передают `filters.period !== null` — без применённого
   * периода чип серый «Период». */
  readonly periodActive: boolean;
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
 * период применён (periodActive; на дефолте «весь период» — карте #669 —
 * все экраны передают `filters.period !== null`, без периода чип серый
 * «Период»), «Категория» — синий только с активным фильтром. Чипы открывают
 * свои шиты выбора (#477, Figma 1492-41825). Глобальная лента (#541)
 * добавляет между ними чип «Объект» — лейбл «Все объекты»/«1 объект»/
 * «N объектов», синий с активным фильтром.
 */
export function OperationsFilterChips({
  periodLabel,
  periodActive,
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

/**
 * Чип периода: с `onOpen` — кнопка открытия канонного пикера (страницы
 * выбора категорий — глобальная #544 и объектная #477; решение владельца
 * 01.10: период меняется на месте, применение возвращает к выбору
 * категории), без — дисплейный span (скелетоны #474/#544). С применённым
 * периодом — синий с диапазоном, в дефолте «весь период» — серый
 * «Период» (канон #670).
 */
export function OperationsPeriodChip({
  period,
  onOpen,
}: {
  readonly period: OperationsPeriod | null;
  readonly onOpen?: () => void;
}): JSX.Element {
  const className = clsx(
    "inline-flex h-11 items-center rounded-pill px-5 text-sm font-medium cursor-pointer",
    period !== null ? "bg-primary text-white" : "bg-surface-muted text-content",
  );
  const label = operationsPeriodChipLabel(period);
  if (onOpen === undefined) {
    return (
      <span aria-hidden className={className}>
        {label}
      </span>
    );
  }
  return (
    <button type="button" onClick={onOpen} className={className}>
      {label}
    </button>
  );
}

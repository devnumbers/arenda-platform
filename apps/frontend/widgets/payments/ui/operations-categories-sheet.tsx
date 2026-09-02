'use client';

import { useState, type JSX } from 'react';
import { Cancel } from '@/shared/assets/icons';
import type { OperationsCategoryRow } from '@/features/payments';
import {
  categoryStyle,
  CategoryIcon,
} from '@/features/payment-categories';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { Button, Checkbox, IconButton, ListRow, Modal, ModalContent } from '@/shared/ui/design';

export type OperationsCategoriesSheetProps = {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  /** Категории с операциями за период (разбивка сводки #473) — строки
   * шита; правило владельца: категорий без операций в списке нет. */
  readonly rows: ReadonlyArray<OperationsCategoryRow>;
  /** Применённый выбор — исходное состояние черновика открытого шита. */
  readonly selected: ReadonlyArray<string>;
  /** Лейбл контекстного чипа периода («1 — 30 ноя»), Figma 1506-72116. */
  readonly periodLabel: string;
  readonly onApply: (slugs: ReadonlyArray<string>) => void;
};

/**
 * Полноэкранный шит «Выбрать категорию» (#477, Figma 1506-72116,
 * 1510-74149): сверху контекстные чипы текущего периода и «Все категории»,
 * ниже строки «иконка + название + сумма за период + чекбокс» с
 * мультивыбором. «Выбрать» (виден при непустом выборе, как в макете)
 * применяет слаги контрактом `category`; крестик/свайп отбрасывают
 * изменения. Суммы строк — из сводки периода, поэтому выбор всегда
 * согласован с карточками и списком экрана.
 */
export function OperationsCategoriesSheet({
  open,
  onOpenChange,
  rows,
  selected,
  periodLabel,
  onApply,
}: OperationsCategoriesSheetProps): JSX.Element {
  // Черновик живёт от монтирования до монтирования: шит получает новый key
  // на каждую сессию открытия (OperationsFiltersArea), поэтому useState
  // инициализируется свежим применённым выбором без эффектов.
  const [draft, setDraft] = useState<ReadonlyArray<string>>(selected);

  const toggle = (slug: string): void => {
    setDraft(draft.includes(slug) ? draft.filter((candidate) => candidate !== slug) : [...draft, slug]);
  };

  const apply = (): void => {
    onApply(draft);
    onOpenChange(false);
  };

  return (
    <Modal open={open} onOpenChange={onOpenChange}>
      <ModalContent
        title="Выбрать категорию"
        titleSrOnly
        fullScreen
        footer={
          draft.length > 0 || selected.length > 0 ? (
            // Пустой черновик при применённом фильтре — легитимное применение
            // «Все категории»: возврат к дефолту должен быть достижим.
            <Button className="w-full" onClick={apply} aria-label="Выбрать категории">
              Выбрать
            </Button>
          ) : undefined
        }
      >
        <div className="flex items-center justify-between gap-4">
          <IconButton icon={<Cancel />} label="Закрыть" onClick={() => onOpenChange(false)} />
          <span className="text-xl font-semibold leading-6 text-content">Выбрать категорию</span>
          <span className="w-11 shrink-0" aria-hidden />
        </div>

        <div className="flex flex-wrap gap-1.5">
          <span aria-hidden className="inline-flex h-11 items-center rounded-pill bg-primary px-5 text-sm font-medium text-white">
            {periodLabel}
          </span>
          <span aria-hidden className="inline-flex h-11 items-center rounded-pill bg-surface-muted px-5 text-sm font-medium text-content">
            Все категории
          </span>
        </div>

        {rows.length === 0 ? (
          <p className="px-6 py-8 text-center text-base leading-[18px] text-content-secondary">
            В этом периоде нет операций
          </p>
        ) : (
          <div className="flex flex-col">
            {rows.map((row) => {
              const style = categoryStyle('default', row.slug);
              return (
                <ListRow
                  key={row.slug}
                  leading={<CategoryIcon icon={style.icon} color={style.color} />}
                  title={row.label}
                  value={formatMoneyKopecks(row.totalKopecks)}
                  trailing={
                    // Клик по чекбоксу не должен дощёлкивать до строки —
                    // иначе toggle сработает дважды (чекбокс + строка).
                    <Checkbox
                      aria-label={`Категория ${row.label}`}
                      checked={draft.includes(row.slug)}
                      onCheckedChange={() => toggle(row.slug)}
                      onClick={(event) => event.stopPropagation()}
                    />
                  }
                  onSelect={() => toggle(row.slug)}
                />
              );
            })}
          </div>
        )}
      </ModalContent>
    </Modal>
  );
}

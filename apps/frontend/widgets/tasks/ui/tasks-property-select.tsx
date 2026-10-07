'use client';

import type { JSX } from 'react';
import { BoldObjects, Cancel, Check, CheckBoxFalse, CheckBoxTrue } from '@/shared/assets/icons';
import { Button, CircleIcon, IconButton, PageContent, Skeleton, StickyBottomBar, TopNav, TopNavTitle } from '@/shared/ui/design';
import { EMPTY_TASKS_FEED_FILTER, type TasksFeedFilter } from '@/features/tasks';
import { useProperties } from '@/features/properties';
import { propertyTypeIcons, type PropertyType } from '@/entities/property';

/** Черновик фильтра ленты — структурно тот же срез, что и разобранный
 * URL-фильтр (TasksFeedFilter): «Общие задачи» + объекты, оба пустые —
 * «Все задачи» (решение владельца 2026-09-07). Строгий черновик живёт
 * в состоянии страницы, URL трогает только «Выбрать»/✓. */
export type TasksFilterDraft = TasksFeedFilter;

/** Пустой черновик = «Все задачи». */
export const EMPTY_TASKS_FILTER_DRAFT = EMPTY_TASKS_FEED_FILTER;

/**
 * Страница «Выбрать объект» — фильтр глобальной ленты «Задачи» (карта
 * #518, тикеты #524/#547, Figma 1726-88880): тот же каркас, что у выбора
 * объекта контакта (#509/#510) — URL не меняется, черновик живёт в
 * состоянии. Мультивыбор чекбоксами (решение владельца 2026-09-05), выбор
 * применяется «Выбрать»/✓, ✕ отбрасывает. Первая строка — «Все задачи»
 * (без подписи, отмечена, когда черновик пуст; тап очищает черновик), под
 * разделителем — список: «Общие задачи / Не привязана к объекту» —
 * безобъектная книга читателя, свободно совмещается с объектами — union-фид
 * (решение владельца 2026-09-07) — и объекты. У объектов имя и адрес, с
 * фото-аватаром или серым домом, архивов в списке нет (решение 9 #522).
 */
export function TasksPropertySelectPage({
  draft,
  onDraftChange,
  onApply,
  onDismiss,
}: {
  readonly draft: TasksFilterDraft;
  readonly onDraftChange: (draft: TasksFilterDraft) => void;
  readonly onApply: () => void;
  readonly onDismiss: () => void;
}): JSX.Element {
  const propertiesQuery = useProperties();
  const properties = (propertiesQuery.data ?? []).filter(
    (property) => property.status !== 'archived',
  );

  const toggle = (propertyId: string): void => {
    onDraftChange(
      draft.propertyIds.includes(propertyId)
        ? { ...draft, propertyIds: draft.propertyIds.filter((id) => id !== propertyId) }
        : { ...draft, propertyIds: [...draft.propertyIds, propertyId] },
    );
  };

  return (
    <>
      <TopNav
        leading={
          <IconButton icon={<Cancel />} label="Отменить выбор объекта" onClick={onDismiss} />
        }
        trailing={
          <IconButton icon={<Check />} label="Применить выбор объекта" onClick={onApply} />
        }
      >
        <TopNavTitle title="Выбрать объект" />
      </TopNav>

      <PageContent>
        <div role="group" aria-label="Фильтр задач" className="px-6">
          <ObjectRowButton
            title="Все задачи"
            isAll
            checked={draft.propertyIds.length === 0 && !draft.withoutProperty}
            onCheck={() => onDraftChange(EMPTY_TASKS_FILTER_DRAFT)}
          />
          {/* Разделитель — сразу под «Все задачи» (макет 1726:88888);
              «Общие задачи» идут в списке вместе с объектами. */}
          <div aria-hidden className="h-px bg-surface-muted" />
          {propertiesQuery.isPending && <ObjectRowsSkeleton />}
          {propertiesQuery.isError && <ObjectLoadErrorCard onRetry={() => void propertiesQuery.refetch()} />}
          <ObjectRowButton
            title="Общие задачи"
            subtitle="Не привязана к объекту"
            isAll
            checked={draft.withoutProperty}
            onCheck={() => onDraftChange({ ...draft, withoutProperty: !draft.withoutProperty })}
          />
          {properties.map((property) => (
            <ObjectRowButton
              key={property.id}
              title={property.name}
              subtitle={property.address}
              photoUrl={property.photoUrl ?? undefined}
              type={property.type}
              checked={draft.propertyIds.includes(property.id)}
              onCheck={() => toggle(property.id)}
            />
          ))}
        </div>
      </PageContent>

      <StickyBottomBar>
        <Button className="w-full" onClick={onApply}>
          Выбрать
        </Button>
      </StickyBottomBar>
    </>
  );
}

/** Строка-чекбокс страницы «Выбрать объект» (компонент Figma «Row Button»,
 * 936:39347): аватар Category Icon 44px (#F3F4F6 + белое кольцо 2.5px) с
 * иконкой 24 — Icon/Bold/Objects у «Все задачи»/«Общие задачи», глиф типа
 * объекта (Category Icon, карта #1217) у объектов, или фото; заголовок
 * 16/500, опциональная подпись 14 #6F787C, чекбокс Selection Button
 * справа (Figma 1031:21053, Variant=Checkbox; у «Все задачи» подписи
 * нет — макет 1726:88886). */
function ObjectRowButton({
  title,
  subtitle,
  photoUrl,
  type,
  isAll = false,
  checked,
  onCheck,
}: {
  readonly title: string;
  readonly subtitle?: string;
  readonly photoUrl?: string;
  readonly type?: PropertyType;
  readonly isAll?: boolean;
  readonly checked: boolean;
  readonly onCheck: () => void;
}): JSX.Element {
  // Выборка из статичного реестра, не вызов: react-hooks/static-components.
  const Glyph = type !== undefined ? propertyTypeIcons[type] : BoldObjects;
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      onClick={onCheck}
      className="flex w-full cursor-pointer items-center gap-3 py-3 text-left outline-none transition-opacity hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface active:opacity-80"
    >
      <CircleIcon variant="white" aria-hidden className="relative overflow-hidden rounded-full">
        {photoUrl !== undefined ? (
          <img src={photoUrl} alt="" className="h-full w-full object-cover" />
        ) : isAll ? (
          <BoldObjects className="h-6 w-6 text-content-tertiary" />
        ) : (
          <Glyph className="h-6 w-6 text-content-tertiary" />
        )}
      </CircleIcon>
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="truncate text-base font-medium leading-[18px] text-content">{title}</span>
        {subtitle !== undefined && (
          <span className="truncate text-sm leading-4 text-content-secondary">{subtitle}</span>
        )}
      </span>
      {checked ? (
        <CheckBoxTrue className="h-6 w-6 shrink-0" aria-hidden />
      ) : (
        <CheckBoxFalse className="h-6 w-6 shrink-0" aria-hidden />
      )}
    </button>
  );
}

/** Карточка ошибки загрузки объектов (состояние списков, DESIGN.md §7):
 * заголовок, серое пояснение и «Повторить». Общая со страницей выбора
 * объекта формы создания (#525). */
export function ObjectLoadErrorCard({ onRetry }: { readonly onRetry: () => void }): JSX.Element {
  return (
    <section className="rounded-card bg-surface-muted px-6 py-6">
      <h2 className="text-base font-medium leading-[18px] text-content">
        Не удалось загрузить объекты
      </h2>
      <p className="mt-2 text-sm leading-4 text-content-secondary">
        Проверьте подключение и попробуйте еще раз
      </p>
      <div className="mt-4">
        <Button size="small" variant="secondary" onClick={onRetry}>
          Повторить
        </Button>
      </div>
    </section>
  );
}

/** Скелет строк объектов на время загрузки списка — каркас ObjectRowButton
 * (аватар 44, заголовок 16/18 + подпись 14/16, зазор 12, py-3 строки).
 * Общий со страницей выбора объекта формы создания (#525) — тот же каркас
 * строк. */
export function ObjectRowsSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col py-3">
      {[0, 1, 2, 3].map((row) => (
        <div key={row} className="flex items-center gap-3 py-3">
          <Skeleton className="h-11 w-11 shrink-0 rounded-full" />
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <Skeleton className="h-[18px] w-2/5" />
            <Skeleton className="h-4 w-3/5" />
          </div>
        </div>
      ))}
    </div>
  );
}

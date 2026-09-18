'use client';

import type { JSX } from 'react';
import { BoldHome, RadioFalse, RadioTrue } from '@/shared/assets/icons';
import { Button, Skeleton } from '@/shared/ui/design';
import { useProperties } from '@/features/properties';
import { filterEditableProperties } from '@/entities/property';

/**
 * Шаг «Выбрать объект» визарда операции (#570, Figma 1858:105011) — только
 * у глобального входа (аннотация фрейма: у входа с объекта этой страницы
 * нет). Каркас выбора объекта задачи/контакта: радио-список объектов,
 * архивные отфильтрованы (решение владельца 08.09), строки без «общей»
 * операции — операция всегда привязана к объекту. Выбор применяется
 * сразу, сабмит «Добавить операцию» — панель шага во flow. Список —
 * только объекты с правом правки (#703): зритель и архив ведут в отказ
 * сервера при сабмите.
 */
export function OperationPropertyStep({
  selectedPropertyId,
  onSelect,
}: {
  readonly selectedPropertyId: string | undefined;
  readonly onSelect: (propertyId: string) => void;
}): JSX.Element {
  const propertiesQuery = useProperties();
  const properties = filterEditableProperties(propertiesQuery.data ?? []);

  return (
    <div role="radiogroup" aria-label="Объект операции" className="px-6 pt-6">
        {propertiesQuery.isPending && <ObjectRowsSkeleton />}
        {propertiesQuery.isError && (
          <ObjectLoadErrorCard onRetry={() => void propertiesQuery.refetch()} />
        )}
        {properties.map((property) => (
          <ObjectRowButton
            key={property.id}
            title={property.name}
            subtitle={property.address}
            photoUrl={property.photos?.[0]?.url}
            checked={selectedPropertyId === property.id}
            onCheck={() => onSelect(property.id)}
          />
        ))}
      </div>
  );
}

/** Строка-радио выбора объекта (компонент Figma «Row Button», каркас
 * «Выбрать объект» задач #525): аватар Category Icon 44px с фото или
 * серым домом, заголовок 16/500, подпись 14, кружок выбора справа. */
function ObjectRowButton({
  title,
  subtitle,
  photoUrl,
  checked,
  onCheck,
}: {
  readonly title: string;
  readonly subtitle: string;
  readonly photoUrl?: string;
  readonly checked: boolean;
  readonly onCheck: () => void;
}): JSX.Element {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={checked}
      onClick={onCheck}
      className="flex w-full cursor-pointer items-center gap-3 py-3 text-left outline-none transition-opacity hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface active:opacity-80"
    >
      <span
        aria-hidden
        className="relative flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-full bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]"
      >
        {photoUrl !== undefined ? (
          <img src={photoUrl} alt="" className="h-full w-full object-cover" />
        ) : (
          <BoldHome className="h-6 w-6 text-content-tertiary" />
        )}
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="truncate text-base font-medium leading-[18px] text-content">{title}</span>
        <span className="truncate text-sm leading-4 text-content-secondary">{subtitle}</span>
      </span>
      {checked ? (
        <RadioTrue className="h-6 w-6 shrink-0" aria-hidden />
      ) : (
        <RadioFalse className="h-6 w-6 shrink-0" aria-hidden />
      )}
    </button>
  );
}

function ObjectRowsSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-6 pt-6">
      <Skeleton className="h-11 w-full" />
      <Skeleton className="h-11 w-full" />
      <Skeleton className="h-11 w-full" />
    </div>
  );
}

function ObjectLoadErrorCard({ onRetry }: { readonly onRetry: () => void }): JSX.Element {
  return (
    <section className="rounded-card bg-surface-muted px-6 py-6">
      <h2 className="text-base font-medium leading-[18px] text-content">
        Не удалось загрузить объекты
      </h2>
      <p className="mt-2 text-sm leading-4 text-content-secondary">
        Проверьте подключение и попробуйте еще раз
      </p>
      <div className="mt-4">
        <Button variant="secondary" size="small" onClick={onRetry}>
          Повторить
        </Button>
      </div>
    </section>
  );
}

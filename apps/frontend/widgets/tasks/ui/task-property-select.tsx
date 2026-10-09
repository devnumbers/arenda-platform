'use client';

import { useState, type JSX } from 'react';
import { BoldObjects, Cancel, Check, RadioFalse, RadioTrue } from '@/shared/assets/icons';
import { Button, CircleIcon, IconButton, PageContent, StickyBottomBar, TopNav, TopNavTitle } from '@/shared/ui/design';
import { useProperties } from '@/features/properties';
import { filterEditableProperties, propertyTypeIcons, type PropertyType } from '@/entities/property';
import { ObjectLoadErrorCard, ObjectRowsSkeleton } from './tasks-property-select';

/**
 * Страница «Выбрать объект» формы создания задачи (карта #518, тикет #525,
 * Figma 1726-87754): каркас выбора объекта контакта (#509/#510) — отдельная
 * страница на том же маршруте, URL не меняется, строгий черновик применяют
 * «Выбрать»/✓, ✕ отбрасывает. Радио-выбор одного: «Общая задача / Не
 * привязана к объектам» (null — правило без объекта, ADR 0052) или объект.
 * Архивов в списке нет — решение 9 #522 (как в фильтре ленты #524);
 * объекты — только с правом правки (#703): привязка к чужому
 * зрительскому объекту упёрлась бы в отказ сервера.
 */
export function TaskPropertySelectPage({
  draft,
  onDraftChange,
  onApply,
  onDismiss,
}: {
  readonly draft: string | null;
  readonly onDraftChange: (propertyId: string | null) => void;
  readonly onApply: () => void;
  readonly onDismiss: () => void;
}): JSX.Element {
  const propertiesQuery = useProperties();
  const properties = filterEditableProperties(propertiesQuery.data ?? []);

  return (
    <>
      <TopNav
        leading={
          <IconButton icon={<Cancel />} label="Не менять объект" onClick={onDismiss} />
        }
        trailing={
          <IconButton icon={<Check />} label="Выбрать объект" onClick={onApply} />
        }
      >
        <TopNavTitle title="Выбрать объект" />
      </TopNav>

      <PageContent>
        <div role="radiogroup" aria-label="Привязка задачи к объекту" className="px-6">
          <ObjectRowButton
            title="Общая задача"
            subtitle="Не привязана к объектам"
            isGeneral
            checked={draft === null}
            onCheck={() => onDraftChange(null)}
          />
          {propertiesQuery.isPending && <ObjectRowsSkeleton />}
          {propertiesQuery.isError && <ObjectLoadErrorCard onRetry={() => void propertiesQuery.refetch()} />}
          {properties.length > 0 && <div aria-hidden className="h-px bg-surface-muted" />}
          {properties.map((property) => (
            <ObjectRowButton
              key={property.id}
              title={property.name}
              subtitle={property.address}
              photoUrl={property.photoUrl ?? undefined}
              type={property.type}
              checked={draft === property.id}
              onCheck={() => onDraftChange(property.id)}
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

/** Строка-радио страницы «Выбрать объект» (компонент Figma «Row Button»,
 * 936:39347): аватар Category Icon 44px (#F3F4F6 + белое кольцо 2.5px) с
 * иконкой 24 — Icon/Bold/Objects у «Общей задачи», глиф типа объекта
 * (Category Icon, карта #1217) у объектов, или фото; заголовок 16/500,
 * подпись 14 #6F787C, кружок выбора RadioFalse/RadioTrue справа. */
function ObjectRowButton({
  title,
  subtitle,
  photoUrl,
  type,
  isGeneral = false,
  checked,
  onCheck,
}: {
  readonly title: string;
  readonly subtitle: string;
  readonly photoUrl?: string;
  readonly type?: PropertyType;
  readonly isGeneral?: boolean;
  readonly checked: boolean;
  readonly onCheck: () => void;
}): JSX.Element {
  // Выборка из статичного реестра, не вызов: react-hooks/static-components.
  // Битое фото (404 стрима) откатывается к глифу — канон #1275 (#1286).
  const [photoBroken, setPhotoBroken] = useState(false);

  const Glyph = type !== undefined ? propertyTypeIcons[type] : BoldObjects;
  return (
    <button
      type="button"
      role="radio"
      aria-checked={checked}
      onClick={onCheck}
      className="flex w-full cursor-pointer items-center gap-3 py-3 text-left outline-none transition-opacity hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface active:opacity-80"
    >
      <CircleIcon variant="white" aria-hidden className="relative overflow-hidden rounded-full">
        {photoUrl !== undefined && !photoBroken ? (
          <img
              src={photoUrl}
              alt=""
              className="h-full w-full object-cover"
              onError={() => setPhotoBroken(true)}
            />
        ) : isGeneral ? (
          <BoldObjects className="h-6 w-6 text-content-tertiary" />
        ) : (
          <Glyph className="h-6 w-6 text-content-tertiary" />
        )}
      </CircleIcon>
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

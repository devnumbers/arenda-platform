'use client';

import type { JSX } from 'react';
import { BoldHome, BoldObjects, Cancel, Check, RadioFalse, RadioTrue } from '@/shared/assets/icons';
import { Button, IconButton, PageContent, Skeleton, StickyBottomBar, TopNav, TopNavTitle } from '@/shared/ui/design';
import { useProperties } from '@/features/properties';
import { propertyPermissions } from '@/entities/property';

/**
 * Страница «Выбрать объект» (макет 1539:83846, #509/#510): отдельная
 * страница на том же маршруте, URL не меняется. Строгий черновик: тап по
 * радио-строке меняет только подсветку, применяются «Выбрать»/✓, ✕
 * отбрасывает. Первая строка — «Общий контакт / Не привязан к объектам»
 * (тексты 1:1 из макета), объекты — имя и адрес, с фото-аватаром или
 * серым домом. Объекты — только с правом правки (#703): привязка своего
 * контакта к зрительскому объекту закрыла бы её правку.
 */
export function ContactObjectSelectPage({
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
  const properties = (propertiesQuery.data ?? []).filter((property) =>
    propertyPermissions(property).canEdit,
  );

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
        <div role="radiogroup" aria-label="Привязка контакта к объекту" className="px-6">
          <ObjectRowButton
            title="Общий контакт"
            subtitle="Не привязан к объектам"
            isGeneral
            checked={draft === null}
            onCheck={() => onDraftChange(null)}
          />
          {propertiesQuery.isPending && <ObjectRowsSkeleton />}
          {propertiesQuery.isError && (
            <section className="rounded-card bg-surface-muted px-6 py-6">
              <h2 className="text-base font-medium leading-[18px] text-content">
                Не удалось загрузить объекты
              </h2>
              <p className="mt-2 text-sm leading-4 text-content-secondary">
                Проверьте подключение и попробуйте еще раз
              </p>
              <div className="mt-4">
                <Button
                  size="small"
                  variant="secondary"
                  onClick={() => void propertiesQuery.refetch()}
                >
                  Повторить
                </Button>
              </div>
            </section>
          )}
          {properties.length > 0 && (
            <div aria-hidden className="h-px bg-surface-muted" />
          )}
          {properties.map((property) => (
            <ObjectRowButton
              key={property.id}
              title={property.name}
              subtitle={property.address}
              photoUrl={property.photos?.[0]?.url}
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
 * иконкой 24 — Icon/Bold/Objects у «Общего контакта», Icon/Bold/Home
 * (BoldHome) у объектов, или фото; заголовок 16/500, подпись 14 #6F787C,
 * кружок выбора RadioFalse/RadioTrue справа. */
function ObjectRowButton({
  title,
  subtitle,
  photoUrl,
  isGeneral = false,
  checked,
  onCheck,
}: {
  readonly title: string;
  readonly subtitle: string;
  readonly photoUrl?: string;
  readonly isGeneral?: boolean;
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
        ) : isGeneral ? (
          <BoldObjects className="h-6 w-6 text-content-tertiary" />
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

/** Скелет строк объектов на время загрузки списка. */
function ObjectRowsSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-6 py-6">
      {[0, 1, 2, 3].map((row) => (
        <div key={row} className="flex items-center gap-2">
          <Skeleton className="h-12 w-12 rounded-full" />
          <div className="flex flex-1 flex-col gap-2">
            <Skeleton className="h-4 w-2/5" />
            <Skeleton className="h-3.5 w-3/5" />
          </div>
        </div>
      ))}
    </div>
  );
}

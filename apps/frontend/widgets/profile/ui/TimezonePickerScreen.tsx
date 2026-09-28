'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { filterPickerOptions } from '@/shared/ui/design/picker-filter';
import { IconButton, ListRow, PageContent, SearchField, TopNav } from '@/shared/ui/design';
import { useMe } from '@/features/auth';
import { useUpdateMe } from '@/features/profile';
import { timezoneOptions } from '../lib/timezone';

/**
 * Экран «Часовой пояс» (карта #591, тикет #594; Figma 1869-70821):
 * страница-маршрут с поисковой шапкой канона #543 — «←» в левом слоте
 * ведёт назад на аккаунт (history-first, фолбэк ROUTES.profileAccount),
 * поле получает программный фокус, ввод фильтрует список по городу или
 * смещению (канон filterPickerOptions #505, без дебаунса — список
 * локальный). Ниже — все зоны территории РФ строками Row Button (ListRow)
 * «Город (UTC±N)»; выбранная зона — синий заголовок #2B7FFF
 * (text-primary), как в макете (1869:71099), без галки. Состав списка —
 * полный набор UTC+2…+12 против фрагмента макета UTC+2…+7 (открытый
 * вопрос тикета — предложен полный, решается владелец на приёмке).
 * Тап по строке сохраняет зону тихим PATCH /me (как автосохранение
 * аккаунта #593) и возвращает на аккаунт; повторный тап по сохранённой
 * зоне без запроса просто закрывает пикер.
 */
export function TimezonePickerScreen(): JSX.Element {
  const router = useRouter();
  const updateMe = useUpdateMe();
  const { data: me } = useMe();
  const [search, setSearch] = useState('');
  const searchInputRef = useRef<HTMLInputElement | null>(null);

  // Открытие экрана сразу делает поле активным (программный фокус —
  // устоявшийся a11y-паттерн поисковой шапки, как у поиска контактов).
  useEffect(() => {
    searchInputRef.current?.focus();
  }, []);

  const visible = filterPickerOptions(timezoneOptions, search);
  const selected = me?.timezone ?? null;
  const closePicker = (): void => goBack(router, ROUTES.profileAccount);

  const handleSelect = (value: string): void => {
    if (value === selected) {
      closePicker();
      return;
    }
    updateMe.mutate({ timezone: value }, {
      onSuccess: () => {
        notify.scenarios.profile.personalDataSaved();
        closePicker();
      },
      onError: (error) => notify.scenarios.profile.personalDataSaveError(error),
    });
  };

  return (
    <>
      <TopNav
        variant="search"
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={closePicker}
          />
        }
      >
        <SearchField
          ref={searchInputRef}
          aria-label="Поиск часового пояса"
          placeholder="Часовой пояс"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          onClear={() => setSearch('')}
        />
      </TopNav>

      <PageContent>
        <div className="flex flex-col">
          {/* me не нужен для работы списка — только для подсветки выбора;
           * пока /me в полёте, строки гасятся (защита от двойного PATCH). */}
          {visible.map((option) => (
            <ListRow
              key={option.value}
              title={option.label}
              // Строки текстовые, без leading-слота: min-h держит зону тапа
              // ≥44 (§14) и шаг макета 1869-70821 ~52.
              className="min-h-13"
              titleClassName={option.value === selected ? 'text-primary' : undefined}
              disabled={updateMe.isPending}
              onSelect={() => handleSelect(option.value)}
            />
          ))}
          {visible.length === 0 && (
            <p className="px-6 py-4 text-sm text-content-tertiary">Ничего не найдено</p>
          )}
        </div>
      </PageContent>
    </>
  );
}

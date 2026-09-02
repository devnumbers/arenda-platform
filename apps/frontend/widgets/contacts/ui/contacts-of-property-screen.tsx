'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { Add, ArrowLeft, BoldUser } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useDebounce } from '@/shared/lib/hooks/useDebounce';
import { useContacts } from '@/features/contacts';
import { useProperty } from '@/features/properties';
import {
  IconButton,
  ListRow,
  PageContent,
  RoundActionButton,
  SearchField,
  StickyBottomBar,
  TopNav,
} from '@/shared/ui/design';
import { contactRowModel } from '../lib/contact-list-model';
import { ContactsEmptyState, ContactsErrorCard, ContactsSkeleton } from './contacts-states';

/** Задержка дебаунса поиска (мс) — серверный фильтр по ?search=. */
const SEARCH_DEBOUNCE_MS = 300;

/**
 * Экран «Контакты объекта» (#508, карта #503): TopNav в варианте поиска
 * (паттерн платёжного поиска), серверный регистронезависимый ?search= по
 * имени, телефону, почте, мессенджеру и роли с дебаунсом, строки ListRow —
 * ФИО + роль, второй строкой телефон. Кнопка «Добавить» видна тому, кто
 * может мутировать (как у платежей: не смотрящий и не архив, #446); ведёт
 * на экран создания — появляется в #509 по порядку приёмки карты.
 */
export function ContactsOfPropertyScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();
  const propertyQuery = useProperty(propertyId);

  const [search, setSearch] = useState('');
  const debouncedSearch = useDebounce(search, SEARCH_DEBOUNCE_MS);
  // Сервер фильтр не нормализует — пробелы по краям срезаем клиентски:
  // запрос из одних пробелов не должен уходить в ?search=.
  const trimmedSearch = debouncedSearch.trim();
  const contactsQuery = useContacts(propertyId, trimmedSearch);

  const contacts = contactsQuery.data ?? [];
  const searching = trimmedSearch.length > 0;

  // Мутационный вход — тому, кому можно мутировать: смотрящий читает без
  // кнопок (как у платежей, история 47), архив read-only (#446). Пока
  // объект не загружен или не загрузился — без кнопки.
  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const role = property?.access?.role;
  const canMutate =
    property !== undefined && role !== undefined && role !== 'viewer' && property.status !== 'archived';

  return (
    <>
      <TopNav
        variant="search"
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.property(propertyId))}
          />
        }
      >
        <SearchField
          aria-label="Поиск контактов"
          placeholder="Поиск"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          onClear={() => setSearch('')}
        />
      </TopNav>

      <PageContent>
        {contactsQuery.isPending ? (
          <ContactsSkeleton />
        ) : contactsQuery.isError ? (
          <ContactsErrorCard onRetry={() => void contactsQuery.refetch()} />
        ) : contacts.length === 0 ? (
          searching ? (
            <ContactsEmptyState
              title="Ничего не нашлось"
              hint="Поиск ищет по имени, телефону, почте, мессенджеру и роли"
            />
          ) : (
            <ContactsEmptyState
              title="Нет контактов"
              hint="Добавьте тех, кто помогает с объектом, — сантехника, управляющую компанию, консьержа"
            />
          )
        ) : (
          <section className="mx-6 rounded-card bg-surface-muted py-2">
            {contacts.map((contact) => {
              const row = contactRowModel(contact);
              return (
                <ListRow
                  key={contact.id}
                  leading={<ContactAvatar />}
                  title={row.title}
                  subtitle={row.subtitle}
                />
              );
            })}
          </section>
        )}
      </PageContent>

      {canMutate && (
        <StickyBottomBar>
          <div className="flex justify-center">
            <RoundActionButton
              variant="primary"
              icon={<Add />}
              caption="Добавить"
              onClick={() => router.push(ROUTES.propertyContactNew(propertyId))}
            />
          </div>
        </StickyBottomBar>
      )}
    </>
  );
}

/** Нейтральный аватар строки: белый круг 44×44 с кантом под серую
 * поверхность (та же геометрия, что у CategoryIcon, резолюция #449). */
function ContactAvatar(): JSX.Element {
  return (
    <span
      aria-hidden
      className="flex h-11 w-11 shrink-0 items-center justify-center rounded-pill bg-surface text-content-tertiary shadow-[0_0_0_2.5px_var(--dl-surface-muted)]"
    >
      <BoldUser className="h-6 w-6" />
    </span>
  );
}

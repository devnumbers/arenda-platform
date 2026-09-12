'use client';

import type { JSX } from 'react';
import { ArrowLeft, Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import {
  HubCollapseAnchor,
  HubTitle,
  IconButton,
  PageContent,
  SearchField,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {
  ContactDetailSkeleton,
  ContactsBookSkeleton,
  ContactsSearchHint,
  ContactsSkeleton,
} from './contacts-states';
import { BookSearchPill, BookSortChip } from './contact-book-screen';

/**
 * Route-loading архетипы зоны контактов (#609): хром экрана — вне фазы
 * загрузки (§7), контент — скелетоны-архетипы #605/#606. Используются как
 * fallback Suspense-границы и как loading.tsx сегмента.
 */

/** Книга контактов: хаб-шапка, пилюля, чип сортировки, скелетон книги. */
export function ContactBookLoading(): JSX.Element {
  return (
    <>
      <TopNav
        mobileWings
        collapse={{
          title: 'Контакты',
          search: { href: ROUTES.contactSearch, label: 'Найти контакт' },
        }}
      />

      <PageContent>
        <HubCollapseAnchor>
          <HubTitle>Контакты</HubTitle>
          <div className="mt-4 mb-6 px-6">
            <BookSearchPill onOpenSearch={() => {}} onCreate={() => {}} />
          </div>
        </HubCollapseAnchor>

        <div className="mb-4 px-6">
          <BookSortChip field="name" order="asc" />
        </div>

        <ContactsBookSkeleton />
      </PageContent>
    </>
  );
}

/** Поиск по книге: поисковая шапка канона #508 и подсказка «по чему
 * ищем» — первый кадр живого экрана до ввода. */
export function ContactSearchLoading(): JSX.Element {
  return (
    <>
      <TopNav
        variant="search"
        leading={<IconButton icon={<ArrowLeft />} label="Закрыть поиск" />}
      >
        <SearchField aria-label="Поиск контактов" placeholder="Найти контакт" />
      </TopNav>

      <PageContent>
        <ContactsSearchHint />
      </PageContent>
    </>
  );
}

/** Карточка контакта: шапка подэкрана и скелетон карточки (#606). */
export function ContactDetailLoading(): JSX.Element {
  return (
    <>
      <TopNav leading={<IconButton icon={<ArrowLeft />} label="Назад" />}>
        <TopNavTitle title="Контакт" />
      </TopNav>

      <PageContent>
        <ContactDetailSkeleton />
      </PageContent>
    </>
  );
}

/** Правка контакта: шапка правки без ✓ (как EditHeader живого экрана) и
 * скелетон книги. */
export function ContactEditLoading(): JSX.Element {
  return (
    <>
      <TopNav leading={<IconButton icon={<Cancel />} label="Назад" />}>
        <TopNavTitle title="Изменить контакт" />
      </TopNav>

      <PageContent>
        <ContactsSkeleton />
      </PageContent>
    </>
  );
}

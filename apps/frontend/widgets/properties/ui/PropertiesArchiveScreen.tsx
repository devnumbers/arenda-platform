'use client';

import type { JSX } from 'react';
import { useArchivedProperties } from '@/features/properties';
import { ROUTES } from '@/shared/config/routes';
import { EmptyState, SubScreenShell } from '@/shared/ui/design';
import { archiveCountLabel } from '../lib/archive-count-label';
import { PropertyCard } from './PropertyCard';
import { PropertiesLoading } from './PropertiesLoading';
import { PropertiesErrorState } from './PropertiesErrorState';
import listStyles from './PropertiesPage.module.css';

/**
 * Экран «Архивные объекты» (карта #583, тикет #587; Figma 1603:92102):
 * каркас подэкрана — «Назад» на список объектов, заголовок с
 * подзаголовком-счётчиком «N объектов» (pluralize); карточки анатомии
 * списка с бейджем «В архиве» (вариант archived PropertyCard); тап — на
 * страницу объекта. Порядок — бэковый updated_at DESC (свежее сверху),
 * сортировок и поиска в архиве нет — согласовать на приёмке. Пустое
 * состояние без макета — канон EmptyState, копилка решения владельца на
 * приёмке (иллюстрация пока от списка объектов).
 */
export function PropertiesArchiveScreen(): JSX.Element {
  const { data, isLoading, isFetching, isError, refetch } = useArchivedProperties();

  const loaded = data !== undefined;
  const isEmpty = loaded && data.length === 0;

  return (
    <SubScreenShell
      title="Архивные объекты"
      subtitle={loaded ? archiveCountLabel(data.length) : undefined}
      fallbackHref={ROUTES.properties}
    >
      <div className="flex flex-col pt-6">
        {isLoading && <PropertiesLoading />}

        {!isLoading && isError && (
          <PropertiesErrorState onRetry={() => void refetch()} isLoading={isFetching} />
        )}

        {!isLoading && !isError && isEmpty && (
          <EmptyState
            imageSrc="/images/properties/properties-empty.png"
            imageAlt="В архиве пока пусто"
            title="В архиве пока пусто"
            description="Заархивированные объекты появятся здесь"
            className="pt-6"
          />
        )}

        {loaded && !isEmpty && (
          <ul className={listStyles.list} data-testid="properties-archive-list">
            {data.map((property) => (
              <li key={property.id}>
                <PropertyCard property={property} variant="archived" />
              </li>
            ))}
          </ul>
        )}
      </div>
    </SubScreenShell>
  );
}

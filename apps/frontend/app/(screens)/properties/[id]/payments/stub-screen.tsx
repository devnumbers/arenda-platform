'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { IconButton, PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';

/** Композиция-шаблон экрана нового хрома (#460): локальный TopNav с
 * кнопкой «назад» и заголовком над центрированной колонкой контента.
 * На мобайле этот TopNav — единственная шапка экрана; на десктопе он
 * стоит под глобальным top-header оболочки. */
export function PaymentsStubScreen({ propertyId }: { readonly propertyId: string }): JSX.Element {
  const router = useRouter();

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.property(propertyId))}
          />
        }
      >
        <TopNavTitle title="Платежи объекта" />
      </TopNav>
      <PageContent>
        <section className="px-6">
          <p className="text-base text-content-secondary">
            Здесь появятся платежи и автоплатежи объекта.
          </p>
        </section>
      </PageContent>
    </>
  );
}

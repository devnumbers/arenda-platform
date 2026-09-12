'use client';

import type { JSX } from 'react';
import Image from 'next/image';
import type {
  TariffAboutCardView,
  TariffFeatureRow,
} from '@/widgets/profile/lib/tariff-about';

/** Картинки рядов возможностей (48×48, без скругления — как в макетах);
 * переиспользуются карточками экрана «Выбрать тариф» (#623). */
export const FEATURE_IMAGES: Record<TariffFeatureRow['kind'], string> = {
  objects: '/images/tariff/tariff-about-objects.png',
  sharing: '/images/tariff/tariff-about-sharing.png',
};

/** Карточка тарифа экрана «О тарифе» (#621, макет 1918-73255): серая
 * карточка 32/32, радиус 32, вертикальный ритм 24; ряд — серый лейбл
 * 16/18 и значение H3 20/24. Набор рядов считает либа tariff-about. */
export function TariffAboutCard({
  view,
}: {
  readonly view: TariffAboutCardView;
}): JSX.Element {
  return (
    <section className="flex flex-col gap-6 rounded-[32px] bg-surface-muted p-8">
      <h2 className="m-0 text-2xl font-semibold leading-8 text-content">{view.title}</h2>
      {view.rows.map((row) => (
        <div key={row.label} className="flex flex-col gap-2">
          <p className="m-0 text-base leading-[18px] text-content-secondary">{row.label}</p>
          <p className="m-0 text-xl font-semibold leading-6 text-content">{row.value}</p>
        </div>
      ))}
    </section>
  );
}

/** Карточка «Возможности» (#621, макет 1918-73255): ряд — картинка 48×48
 * (без скругления, как в макете) и колонка заголовок 18/600 + описание
 * 14/16; «Совместный доступ» прикладывается либой только платным. */
export function TariffFeaturesCard({
  rows,
}: {
  readonly rows: readonly TariffFeatureRow[];
}): JSX.Element {
  return (
    <section className="flex flex-col gap-6 rounded-[32px] bg-surface-muted p-8">
      <h2 className="m-0 text-2xl font-semibold leading-8 text-content">Возможности</h2>
      {rows.map((row) => (
        <div key={row.kind} className="flex gap-4">
          <Image
            src={FEATURE_IMAGES[row.kind]}
            alt=""
            width={48}
            height={48}
            className="h-12 w-12 shrink-0"
          />
          <div className="flex flex-col gap-2">
            <p className="m-0 text-lg font-semibold leading-[18px] text-content">{row.title}</p>
            <p className="m-0 text-sm leading-4 text-content-secondary">{row.description}</p>
          </div>
        </div>
      ))}
    </section>
  );
}

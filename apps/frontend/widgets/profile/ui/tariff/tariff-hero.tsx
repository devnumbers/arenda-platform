'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import Image from 'next/image';
import { InfoSmall, SmallArrowRight } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { cn } from '@/shared/lib/cn';
import type { TariffHero as TariffHeroModel } from '@/widgets/profile/lib/tariff-overview';

const heroLinkFocus =
  'outline-none focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface';

/** Hero-карточка главного экрана «Тариф» (#620, Figma 1879-70076 —
 * 1943-115049): скругление 32, паддинг 32, вертикальный ритм 24.
 * Платные тарифы и базовый — фото-фон (imageRef 253b396c…, cover) с белым
 * текстом (подписи 60% opacity); остановленный — серая карточка с тёмным
 * текстом (опечатка макета «Про остановлен » исправлена). Внутри карточки
 * две ссылки-строки: заголовок с шевроном (→ «О тарифе») и нижний ряд —
 * «О тарифе» (grace — CTA «Оплатить тариф» на флоу оплаты, 1917-72464);
 * карточка целиком ссылкой не является, чтобы не вкладывать интерактивы. */
export function TariffHero({ hero }: { readonly hero: TariffHeroModel }): JSX.Element {
  const stopped = hero.kind === 'stopped';

  return (
    <div
      className={cn(
        'relative flex flex-col gap-6 overflow-hidden rounded-[32px] p-8',
        stopped ? 'bg-surface-muted text-content' : 'bg-primary text-white',
      )}
    >
      {!stopped && (
        <Image
          src="/images/tariff/tariff-hero.png"
          alt=""
          fill
          priority
          sizes="(max-width: 607px) calc(100vw - 48px), 512px"
          className="-z-10 object-cover"
        />
      )}

      <NextLink
        href={ROUTES.profileTariffAbout}
        className={cn('flex items-center justify-between gap-4', heroLinkFocus)}
      >
        <h2 className="m-0 text-2xl font-semibold leading-8">{hero.title}</h2>
        <SmallArrowRight className="shrink-0" aria-hidden />
      </NextLink>

      {(hero.kind === 'paid' || hero.kind === 'grace') && (
        <div className="flex flex-col gap-2">
          <p className="m-0 text-lg font-semibold leading-6">{hero.priceLine}</p>
          {hero.subLine !== undefined && (
            <p className="m-0 text-base font-medium leading-[18px] opacity-60">{hero.subLine}</p>
          )}
        </div>
      )}

      {hero.kind === 'basic' && (
        <p className="m-0 text-base font-medium leading-[18px] opacity-60">{hero.subLine}</p>
      )}

      {hero.kind === 'stopped' && (
        <div className="flex flex-col gap-2">
          {hero.dateLine !== undefined && (
            <p className="m-0 text-lg font-semibold leading-6">{hero.dateLine}</p>
          )}
          <p className="m-0 text-base font-medium leading-[18px] opacity-60">{hero.subLine}</p>
        </div>
      )}

      <NextLink
        href={hero.kind === 'grace' ? ROUTES.profileTariffChange : ROUTES.profileTariffAbout}
        className={cn('flex items-center gap-1.5 text-base font-medium leading-[18px]', heroLinkFocus)}
      >
        <InfoSmall className="h-4 w-4 shrink-0" aria-hidden />
        {hero.kind === 'grace' ? 'Оплатить тариф' : 'О тарифе'}
      </NextLink>
    </div>
  );
}

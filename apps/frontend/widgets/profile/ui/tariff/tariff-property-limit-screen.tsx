'use client';

import type { JSX } from 'react';
import Image from 'next/image';
import { useRouter } from 'next/navigation';
import { Button, StickyBottomBar } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useSubscription } from '@/features/subscription';
import {
  PROPERTY_LIMIT_TITLE,
  propertyLimitUpgradeText,
} from '@/widgets/profile/lib/tariff-property-limit';

/**
 * Экран-интерстишал «Превышен лимит объектов» (карта #1459, тикет #1469,
 * макет Figma 3516-79702): иллюстрация, заголовок и адаптивный по тарифу
 * текст (развязка — propertyLimitUpgradeText, источник тарифа — кэш
 * GET /subscription; подписки нет — basic); внизу «Выбрать тариф» →
 * страница выбора тарифа и «Отменить» → назад. Компоновка — каркас
 * SubScreenShell страницы (шапка «Тариф»); контент прижат кверху —
 * отступ 96 от шапки задаёт страница (правка владельца 16.10, без
 * вертикального центрирования). Входы экрана — «+» хаба
 * объектов (openCreate) и визард создания (402 property_limit_reached,
 * #1468); лейбл кнопки хаба и suspended-поверхности экран не трогает.
 */
export function TariffPropertyLimitScreen(): JSX.Element {
  const router = useRouter();
  const subscriptionQuery = useSubscription();
  const description = propertyLimitUpgradeText(subscriptionQuery.data?.tariff.name);

  return (
    <>
      {/* Иллюстрация 3518:80127 — экспорт композиции одним PNG 2x
       * (домик 128 + бейдж с домиками в круге 72 с тенью). Слот 148×148
       * держит видимый блок макета; PNG несёт поля тени (2/4 слева-
       * сверху, вправо-вниз — разлёт), поэтому смещён за границы слота.
       * Отступ 96 от шапки до слота задаёт страница (pt-24). */}
      <div className="flex flex-col items-center">
        <div className="relative h-[148px] w-[148px]">
          <Image
            src="/images/tariff/tariff-property-limit.png"
            alt=""
            width={172}
            height={180}
            sizes="172px"
            quality={90}
            className="absolute left-[-2px] top-[-4px] h-[180px] w-[172px] max-w-none"
          />
        </div>
        <h2 className="mt-6 text-[24px] font-semibold leading-7 text-content">
          {PROPERTY_LIMIT_TITLE}
        </h2>
        <p className="mt-3 max-w-[360px] text-center text-base leading-[18px] text-content-secondary">
          {description}
        </p>
      </div>

      <StickyBottomBar>
        <div className="flex flex-col gap-2">
          <Button onClick={() => router.push(ROUTES.profileTariffChange)}>
            Выбрать тариф
          </Button>
          <Button variant="secondary" onClick={() => goBack(router, ROUTES.properties)}>
            Отменить
          </Button>
        </div>
      </StickyBottomBar>
    </>
  );
}

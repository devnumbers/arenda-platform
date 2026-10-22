'use client';

import type { JSX } from 'react';
import Image from 'next/image';
import { useRouter } from 'next/navigation';
import { Button, StickyBottomBar } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { goBack, pushPage } from '@/shared/lib/navigation';
import { SHARING_TITLE, SHARING_UPGRADE_TEXT } from '@/widgets/profile/lib/tariff-sharing';

/**
 * Экран-интерстишал «Совместный доступ недоступен» (карта #1459, тикет
 * #1470, макет Figma 3518-80047): иллюстрация, заголовок и один статичный
 * текст описания (в тикете; гейты ведут сюда только basic — владелец из
 * proxy и разводки шаринга, участник из платной ленты истории #1465;
 * адаптивность #1469 не нужна); внизу «Выбрать тариф» → страница выбора
 * тарифа и «Отменить» → назад. Компоновка — каркас SubScreenShell
 * страницы (шапка «Повысить тариф» — по макету); контент прижат кверху —
 * отступ 96 от шапки задаёт страница (правка владельца 16.10 из #1469,
 * без вертикального центрирования). Входы — proxy-цели реестра
 * paid-sections (deep-маршруты участников, лента истории), апселл-CTA
 * хаба /participants и экранная разводка клика «Совместного доступа»
 * (#1463).
 */
export function TariffSharingScreen(): JSX.Element {
  const router = useRouter();

  return (
    <>
      {/* Иллюстрация 3518:80115 — экспорт композиции одним PNG 2x
       * (домик 128 + бейдж с участниками в круге 72 с тенью). Слот
       * 148×148 держит видимый блок макета; PNG несёт поля тени (2/4
       * слева-сверху, вправо-вниз — разлёт), поэтому смещён за границы
       * слота — геометрия соседнего экрана #1469, экспорт той же формы.
       * Отступ 96 от шапки до слота задаёт страница (pt-24). */}
      <div className="flex flex-col items-center">
        <div className="relative h-[148px] w-[148px]">
          <Image
            src="/images/tariff/tariff-sharing.webp"
            alt=""
            width={172}
            height={180}
            sizes="344px"
            quality={90}
            className="absolute left-[-2px] top-[-4px] h-[180px] w-[172px] max-w-none"
          />
        </div>
        <h2 className="mt-6 text-center text-[24px] font-semibold leading-7 text-content">
          {SHARING_TITLE}
        </h2>
        <p className="mt-4 max-w-[329px] text-center text-base leading-[18px] text-content-secondary">
          {SHARING_UPGRADE_TEXT}
        </p>
      </div>

      <StickyBottomBar>
        <div className="flex flex-col gap-2">
          <Button onClick={() => pushPage(router, ROUTES.profileTariffChange)}>
            Выбрать тариф
          </Button>
          <Button variant="secondary" onClick={() => goBack(router, ROUTES.participants)}>
            Отменить
          </Button>
        </div>
      </StickyBottomBar>
    </>
  );
}

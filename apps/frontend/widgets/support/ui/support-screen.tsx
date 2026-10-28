'use client';

import type { JSX } from 'react';
import Image from 'next/image';
import { Check, Copy } from '@/shared/assets/icons';
import { SUPPORT_EMAIL, SUPPORT_TELEGRAM_URL } from '@/shared/config/support';
import { useCopiedHint } from '@/shared/lib/hooks/use-copied-hint';
import { Button, buttonVariants, HubTitle, PageContent, TopNav } from '@/shared/ui/design';

/**
 * Экран-хаб «Поддержка» (макеты 40000074:80636 — мобайл, 28.10; десктоп —
 * канон §3, одноколоночный): хаб-шапка с крыльями (TopNav mobileWings) и
 * H1 «Поддержка» в строке h-8 — топ заголовка ровно 24 от хедера (96),
 * как у хабов без кнопки в строке (#865). Дальше колонка макета:
 * иллюстрация 128 (public/images/support/support-hero.webp, слот 2× —
 * суперсэмплинг канона EmptyState) на 56 от строки заголовка (24+32
 * паддинги макета), блок «Свяжитесь с нами» 24/28 + подпись 16/18
 * (зазор 24), кнопки (зазор 48): Primary Small «Написать в Телеграм» —
 * внешняя ссылка в новой вкладке, hug-ширины, и Secondary Small
 * hello@rentlee.ru с копированием в буфер — тянется в ширину колонки
 * кнопок (alignSelf stretch), обе radius="m" (12) по макету. Модалка
 * «Связаться с нами» (#766) осталась только на шагах входа.
 */
export function SupportScreen(): JSX.Element {
  const { copied, copy } = useCopiedHint();

  return (
    <>
      <TopNav mobileWings />

      <PageContent>
        <div className="flex h-8 items-center">
          <HubTitle>Поддержка</HubTitle>
        </div>

        <div className="flex flex-col items-center px-8 pt-14 text-center">
          <Image
            src="/images/support/support-hero.webp"
            alt=""
            width={128}
            height={128}
            sizes="256px"
            quality={90}
            priority
            className="h-32 w-32"
          />

          <div className="mt-6 flex flex-col items-center gap-2">
            <h2 className="m-0 text-[24px] font-semibold leading-7 text-content">
              Свяжитесь с нами
            </h2>
            <p className="m-0 text-base leading-[18px] text-content-secondary">
              Напишите нам в Телеграм или на почту
            </p>
          </div>

          {/* Колонка кнопок шириной с самую широкую (w-fit + items-stretch):
           * Telegram задаёт ширину, почта тянется до неё же — как в макете. */}
          <div className="mt-12 flex w-fit flex-col items-stretch gap-2">
            {/* Внешняя ссылка на канонной кнопке: next/link не дружит со
             * Slot — ссылки стилизуются buttonVariants (кнопка.tsx). */}
            <a
              href={SUPPORT_TELEGRAM_URL}
              target="_blank"
              rel="noopener noreferrer"
              className={buttonVariants({ size: 'small', radius: 'm' })}
            >
              Написать в Телеграм
            </a>
            <Button
              type="button"
              variant="secondary"
              size="small"
              radius="m"
              trailingIcon={
                copied ? <Check className="h-6 w-6" aria-hidden /> : <Copy className="h-6 w-6" aria-hidden />
              }
              aria-label={copied ? 'Скопировано' : `Скопировать: ${SUPPORT_EMAIL}`}
              onClick={() => copy(SUPPORT_EMAIL)}
            >
              {SUPPORT_EMAIL}
            </Button>
          </div>
        </div>
      </PageContent>
    </>
  );
}

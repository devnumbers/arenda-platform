'use client';

import type { JSX, ReactNode } from 'react';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@radix-ui/react-collapsible';
import NextLink from 'next/link';
import { SmallArrowDown } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { SUPPORT_EMAIL } from '@/shared/config/support';
import { cn } from '@/shared/lib/cn';

/** FAQ главного экрана «Тариф» (#620, макеты 1879-70076/1929-76367):
 * серая карточка radius 24, заголовок «Частые вопросы» H3, семь
 * вопросов-аккордеонов (строка R/500 16/18 + SmallArrowDown, развёрнут —
 * SmallArrowUp поворотом на 180°), ответ M/400 14/16 серым. Тексты —
 * статика из макета, опечатки исправлены («объекты будет заблокированы»,
 * слитые абзацы, имена тарифов с заглавной); ссылки ответа про возврат —
 * правила (страница оферты) и почта поддержки, синим #2B7FFF. */
type FaqSegment = { readonly text: string } | { readonly link: string; readonly href: string };

type FaqItem = {
  readonly question: string;
  readonly answer: ReadonlyArray<ReadonlyArray<FaqSegment>>;
};

const FAQ_ITEMS: ReadonlyArray<FaqItem> = [
  {
    question: 'Как отключить тариф',
    answer: [
      [
        { text: 'Для этого зайдите в раздел «О тарифе» и нажмите «Отключить тариф». Тариф останется активным до конца оплаченного периода, а после его завершения аккаунт автоматически перейдет на базовый тариф' },
      ],
      [
        { text: 'Обратите внимание: после перехода на базовый тариф вы сохраните доступ только к одному объекту. Данные по остальным объектам не удаляются — они останутся в вашем аккаунте и отправятся в архив. Вы сможете вернуться к ним' },
      ],
    ],
  },
  {
    question: 'Как продлить тариф',
    answer: [
      [
        { text: 'Оплата тарифа продлевается автоматически по окончании текущего периода — списание происходит с привязанной карты' },
      ],
    ],
  },
  {
    question: 'Что будет, если я перестану платить за тариф',
    answer: [
      [
        { text: 'Если оплата не пройдет, мы попробуем списывать сумму один раз в день в течение недели и уведомим вас об этом. Если оплатить подписку так и не получится, аккаунт автоматически перейдет на базовый тариф по окончании оплаченного периода' },
      ],
      [{ text: 'В течение недели объекты будут заблокированы' }],
      [
        { text: 'Все ваши данные — объекты, платежи, история — сохраняются, и доступ к ним восстановится сразу после возобновления подписки' },
      ],
    ],
  },
  {
    question: 'Как добавить больше объектов',
    answer: [
      [
        { text: 'Количество объектов зависит от вашего тарифа: Базовый — 1 объект, Про — 5 объектов, Бизнес — без ограничений' },
      ],
      [
        { text: 'Чтобы добавить объект, перейдите в раздел «Объекты» и нажмите «Создать объект». Если на текущем тарифе свободных слотов не осталось, система предложит перейти на следующий тариф' },
      ],
    ],
  },
  {
    question: 'Как работает совместный доступ',
    answer: [
      [
        { text: 'Совместный доступ доступен на тарифах «Про» и «Бизнес». Вы можете пригласить в свой аккаунт других пользователей — например, супруга, управляющего, бухгалтера' },
      ],
      [
        { text: 'Для этого откройте нужный объект → «Совместный доступ» → «Пригласить» и укажите электронную почту приглашенного. Для каждого участника вы можете настроить роль:' },
      ],
      [{ text: '— Просмотр — видит данные по объекту, но не может их изменять;' }],
      [{ text: '— Редактирование — управляет объектом, но не может его удалить;' }],
      [
        { text: 'Вы можете в любой момент отозвать приглашение или изменить права участника' },
      ],
    ],
  },
  {
    question: 'Нужно ли оплачивать тариф тому, кому я хочу дать доступ',
    answer: [
      [
        { text: 'Нет, оплачивать тариф нужно только вам. Приглашенные пользователи пользуются сервисом бесплатно в рамках вашего тарифа' },
      ],
      [
        { text: 'Единственное ограничение — для пользователя с базовым тарифом: он может работать только с одним объектом. Поэтому пригласить его можно, только если у него нет своих объектов — его единственный «слот» займет ваш объект. Если вы хотите дать доступ к нескольким своим объектам другому пользователю — понадобится тариф «Про» или выше' },
      ],
    ],
  },
  {
    question: 'Как оформить возврат средств',
    answer: [
      [
        { text: 'Перед оформлением заявки рекомендуем ознакомиться с ' },
        { link: 'правилами возврата средств', href: ROUTES.profileTerms },
        { text: ' — там описаны условия, при которых возврат возможен' },
      ],
      [
        { text: 'Напишите в службу поддержки на почту ' },
        { link: SUPPORT_EMAIL, href: `mailto:${SUPPORT_EMAIL}` },
        { text: ' и укажите причину возврата. Мы рассмотрим вашу заявку и вернем деньги на ту же карту, с которой была произведена оплата' },
      ],
    ],
  },
];

export function TariffFaq(): JSX.Element {
  return (
    <section className="rounded-card bg-surface-muted pb-3">
      <h2 className="m-0 px-6 pt-6 text-lg font-semibold leading-6 text-content">
        Частые вопросы
      </h2>
      <div>
        {FAQ_ITEMS.map((item) => (
          <FaqEntry key={item.question} item={item} />
        ))}
      </div>
    </section>
  );
}

function FaqEntry({ item }: { readonly item: FaqItem }): JSX.Element {
  return (
    <Collapsible className="group">
      <CollapsibleTrigger
        className={cn(
          'flex w-full cursor-pointer items-center justify-between gap-4 px-6 py-3 text-left font-sans outline-none',
          'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface rounded-card',
        )}
      >
        <span className="text-base font-medium leading-[18px] text-content">{item.question}</span>
        <SmallArrowDown
          className="shrink-0 text-content-tertiary transition-transform duration-200 group-data-[state=open]:rotate-180"
          aria-hidden
        />
      </CollapsibleTrigger>
      <CollapsibleContent
        className="overflow-hidden px-6 pb-3 data-[state=open]:animate-[faq-open_300ms_var(--dl-ease)] data-[state=closed]:animate-[faq-close_250ms_var(--dl-ease)]"
      >
        <div className="flex flex-col gap-2">
          {item.answer.map((paragraph, index) => (
            <p key={index} className="m-0 text-sm leading-4 text-content-secondary">
              {paragraph.map(renderSegment)}
            </p>
          ))}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

function renderSegment(segment: FaqSegment, index: number): ReactNode {
  if ('link' in segment) {
    if (segment.href.startsWith('/')) {
      return (
        <NextLink key={index} href={segment.href} className="text-primary">
          {segment.link}
        </NextLink>
      );
    }
    return (
      <a key={index} href={segment.href} className="text-primary">
        {segment.link}
      </a>
    );
  }
  return <span key={index}>{segment.text}</span>;
}

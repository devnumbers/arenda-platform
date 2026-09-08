import type { JSX } from 'react';
import { EmptyState, PageContent, TopNav } from '@/shared/ui/design';

/**
 * Страница-заглушка «Участники» (карта #556, тикет #559): пункт «Участники»
 * главной навигации ведёт на живой маршрут; сама фича — совместный доступ к
 * объекту (ADR 0028, «Участник объекта») — вне карты и придет отдельным
 * усилием. Макетов у заглушки нет — канон хаба новых экранов: TopNav с
 * «крыльями» и на мобайле (как лента «Задачи» #523), заголовок раздела 28 и
 * канонный EmptyState на PageContent; иллюстрация — универсальный пустой
 * лого (как у пустой книги объектов).
 */
export function ParticipantsStubScreen(): JSX.Element {
  return (
    <>
      <TopNav mobileWings />

      <PageContent>
        <h1 className="pl-6 text-[28px] font-semibold leading-8 text-content">Участники</h1>

        <EmptyState
          className="mt-6"
          imageSrc="/images/empty-logo.png"
          title="Участники появятся здесь"
          description="Совместный доступ к объектам появится в этом разделе"
        />
      </PageContent>
    </>
  );
}

import type { JSX } from 'react';
import {
  EmptyState,
  HubTitle,
  PageContent,
  TopNav,
} from '@/shared/ui/design';

/**
 * Страница-заглушка «Платежи» (карта #556, тикет #559): пункт «Платежи»
 * главной навигации ведёт на живой маршрут, содержание раздела — сводка
 * платежей по всем объектам — придёт отдельным усилием владельца (решение
 * при чартинге #556). Макетов у заглушки нет — канон хаба новых экранов:
 * TopNav с «крыльями» и на мобайле (как лента «Задачи» #523), заголовок
 * раздела 28 и канонный EmptyState на PageContent; иллюстрация — пустое
 * состояние «Платежей объекта».
 */
export function PaymentsStubScreen(): JSX.Element {
  return (
    <>
      <TopNav mobileWings />

      <PageContent>
        <HubTitle>Платежи</HubTitle>

        <EmptyState
          className="mt-6"
          imageSrc="/images/payments/empty-payments.png"
          title="Платежи появятся здесь"
          description="Сводка платежей по всем объектам появится в этом разделе"
        />
      </PageContent>
    </>
  );
}

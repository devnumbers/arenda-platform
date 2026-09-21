'use client';

import { useState } from 'react';
import {
  DEFAULT_HISTORY_ORDER,
  serializeHistoryOrderToParams,
  type HistoryOrder,
} from './operations-history';
import { useUrlParams } from '@/shared/lib/hooks/use-url-params';

/**
 * Направление «Истории операций» в адресе (pre-merge #785): дефолт —
 * «сначала новые» (макеты #466/#535), переключение пишет патч через
 * useUrlParams (?order=asc, дефолт не пишется — конвенция
 * состояния в адресе), поэтому перезагрузка и шаринг ссылки сохраняют выбор, а
 * записи истории браузера не создаются. Начальное значение приходит с
 * сервера (parseHistoryOrderParams на странице). Сам запрос хук не делает —
 * направление входит в ключ usePaymentOperationsPaged на экране.
 */
export function useHistoryOrder(initialOrder?: HistoryOrder): {
  readonly order: HistoryOrder;
  readonly toggleOrder: () => void;
} {
  const [order, setOrder] = useState<HistoryOrder>(initialOrder ?? DEFAULT_HISTORY_ORDER);
  const { write } = useUrlParams();

  const toggleOrder = (): void => {
    // Инверсия от текущего значения, не от дефолта: не сцеплена с тем,
    // какое из двух направлений дефолтное.
    const next: HistoryOrder = order === 'asc' ? 'desc' : 'asc';
    setOrder(next);
    write(serializeHistoryOrderToParams(next), { own: ['order'] });
  };

  return { order, toggleOrder };
}

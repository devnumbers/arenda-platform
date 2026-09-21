import type { Metadata } from 'next';
import { sanitizeReturnTo } from '@/shared/lib/navigation';
import { parseStringParam } from '@/shared/lib/parse-string-param';
import { PropertyCreateWizardScreen } from '@/widgets/properties';

/** Визард создания объекта (#480): один маршрут в группе (screens) на
 * едином хроме (карта #556); шаги — клиентское состояние флоу. Старый
 * маршрут кабинета заменён этим с сохранением URL.
 * ?returnTo= (#483, контракт возврата в вызывающий флоу): внутренний
 * абсолютный путь; после создания визард уводит туда с параметром
 * propertyId, минуя экран успеха. */

export const metadata: Metadata = {
  title: 'Создание объекта — Рентли',
  description: 'Новый объект: категория, адрес, характеристики',
};

export default async function PropertyNewRoutePage({
  searchParams,
}: PageProps<'/properties/new'>) {
  const { returnTo } = await searchParams;
  const sanitizedReturnTo = sanitizeReturnTo(parseStringParam(returnTo));

  return <PropertyCreateWizardScreen returnTo={sanitizedReturnTo} />;
}

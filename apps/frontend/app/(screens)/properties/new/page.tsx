import type { Metadata } from 'next';
import { sanitizeReturnTo } from '@/shared/lib/navigation';
import { PropertyCreateWizardScreen } from '@/widgets/properties';

/** Визард создания объекта (#480): один маршрут в группе (screens) —
 * хром новых экранов без Sidebar/BottomNav кабинета; шаги — клиентское
 * состояние флоу. Старый маршрут (cabinet)/properties/new заменён этим.
 * ?returnTo= (#483, контракт возврата в вызывающий флоу): внутренний
 * абсолютный путь; после создания визард уводит туда с параметром
 * propertyId, минуя экран успеха. */

export const metadata: Metadata = {
  title: 'Создание объекта — Рентли',
  description: 'Новый объект: категория, адрес, характеристики',
};

export default async function PropertyNewRoutePage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { returnTo } = await searchParams;
  const sanitizedReturnTo = sanitizeReturnTo(typeof returnTo === 'string' ? returnTo : undefined);

  return <PropertyCreateWizardScreen returnTo={sanitizedReturnTo} />;
}

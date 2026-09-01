import type { Metadata } from 'next';
import { PropertyCreateWizardScreen } from '@/widgets/properties';

/** Визард создания объекта (#480): один маршрут в группе (screens) —
 * хром новых экранов без Sidebar/BottomNav кабинета; шаги — клиентское
 * состояние флоу. Старый маршрут (cabinet)/properties/new заменён этим. */

export const metadata: Metadata = {
  title: 'Создание объекта — Рентли',
  description: 'Новый объект: категория, адрес, характеристики',
};

export default function PropertyNewRoutePage() {
  return <PropertyCreateWizardScreen />;
}

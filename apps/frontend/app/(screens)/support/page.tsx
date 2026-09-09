import type { Metadata } from 'next';
import { SupportScreen } from '@/widgets/support';

/** «Поддержка» на едином хроме (карта #556, тикет #567): маршрут переехал
 * из (cabinet) без смены URL — /support остаётся адресом пункта вторичной
 * навигации (пилюля ПК / шит «Еще» мобайла). */
export const metadata: Metadata = {
  title: 'Поддержка — Рентли',
  description: 'Контакты службы поддержки',
};

export default function SupportRoute() {
  return <SupportScreen />;
}

import type { ReactNode } from 'react';
import { PropertyParticipantsGate } from '@/widgets/participants';

// Клиентская страховка тарифного гейта шаринга (#1463) живёт в
// виджете-гейте: редирект владельца на basic с любого маршрута префикса
// (список, приглашение, права) на интерстишал /profile/tariff/sharing
// после загрузки роли и тарифа. Лейаут — серверный каркас, children
// рендерятся как есть.
export default function PropertyParticipantsLayout({
  children,
}: {
  readonly children: ReactNode;
}) {
  return <PropertyParticipantsGate>{children}</PropertyParticipantsGate>;
}

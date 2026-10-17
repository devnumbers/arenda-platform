import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { TariffPropertyLimitScreen } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Превышен лимит объектов — Рентли',
  description: 'Лимит активных объектов тарифа исчерпан — повысьте тариф, чтобы добавить больше',
};

/** Страница-интерстишал «Превышен лимит объектов» (#1469, макет
 * 3516-79702): каркас подэкрана с шапкой «Тариф» (решение владельца
 * в тикете #1469; в макете шапка «Повысить тариф») и «Назад»
 * history-first с фолбэком на хаб объектов. Входы — «+» хаба объектов
 * при исчерпанном лимите и визард создания (402
 * property_limit_reached, #1468). Контент прижат кверху — отступ 96
 * от шапки, как padding 96px 32px контентного фрейма макета (правка
 * владельца 16.10: без вертикального центрирования — center инстанса
 * Page Content макета не воспроизводить, совпадение с отступом было
 * верно только на высоте макетного фрейма); снизу воздух до панели. */
export default function TariffPropertyLimitPage() {
  return (
    <SubScreenShell
      title="Тариф"
      fallbackHref={ROUTES.properties}
      contentClassName="pt-24"
    >
      <TariffPropertyLimitScreen />
    </SubScreenShell>
  );
}

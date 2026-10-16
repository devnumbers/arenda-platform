import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { TariffSharingScreen } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Совместный доступ недоступен — Рентли',
  description: 'Совместный доступ — платная функция: повысьте тариф, чтобы приглашать участников',
};

/** Страница-интерстишал «Совместный доступ недоступен» (#1470, макет
 * 3518-80047): каркас подэкрана с шапкой «Повысить тариф» (по макету —
 * в отличие от #1469, где владелец заменил её на «Тариф») и «Назад»
 * history-first с фолбэком на хаб участников — корень секции, откуда
 * приходят входы. Входы — proxy-цели реестра paid-sections
 * (deep-маршруты участников, лента истории: ретаргет #1470), апселл-CTA
 * хаба /participants и экранная разводка клика «Совместного доступа»
 * (#1463). Контент прижат кверху — отступ 96 от шапки, как padding
 * 96px 32px контентного фрейма макета (правка владельца 16.10 из #1469:
 * без вертикального центрирования); снизу воздух до панели. */
export default function TariffSharingPage() {
  return (
    <SubScreenShell
      title="Повысить тариф"
      fallbackHref={ROUTES.participants}
      contentClassName="pt-24"
    >
      <TariffSharingScreen />
    </SubScreenShell>
  );
}

import type { Metadata } from 'next';
import { LegalDocument } from '@/widgets/profile';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';

export const metadata: Metadata = {
  title: 'Публичная оферта — Рентли',
  description: 'Условия публичной оферты',
};

/** Третий документ «Информации» (макет 1804-104527, решение владельца
 * 28.09): бар только с «Назад», крупный h1 в контенте; текст пока
 * типовой-заглушка (LegalDocument). */
export default function OfferPage() {
  return (
    <SubScreenShell title="" fallbackHref={ROUTES.profileInfo}>
      <LegalDocument title="Публичная оферта" />
    </SubScreenShell>
  );
}

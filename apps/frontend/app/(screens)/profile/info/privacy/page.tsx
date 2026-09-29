import type { Metadata } from 'next';
import { LegalDocument } from '@/widgets/profile';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';

export const metadata: Metadata = {
  title: 'Политика обработки персональных данных — Рентли',
  description: 'Политика обработки персональных данных',
};

/** Страница документа (макет 2349-68011): бар только с «Назад» (тайтл в
 * баре пустой), крупный h1 в контенте; текст пока типовой-заглушка
 * (LegalDocument). */
export default function PrivacyPage() {
  return (
    <SubScreenShell title="" fallbackHref={ROUTES.profileInfo}>
      <LegalDocument title="Политика обработки персональных данных" />
    </SubScreenShell>
  );
}

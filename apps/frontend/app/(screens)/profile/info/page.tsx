import type { Metadata } from 'next';
import NextLink from 'next/link';
import { SmallArrowRight, RentlyLockup } from '@/shared/assets/icons';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { SUPPORT_EMAIL } from '@/shared/config/support';

export const metadata: Metadata = {
  title: 'Информация — Рентли',
  description: 'Правовая информация и документы',
};

/** Экран «Информация» (макет 1804-104527, решение владельца 28.09 после
 * аудита #877): в баре только «Назад» (тайтла нет), бренд-локап, три
 * документа (политика / соглашение / оферта — строки-карточки radius 24
 * с шевроном, зазор 8), ниже почта поддержки 20/24 синим и «© Рентли
 * {год}» 16/18 серым; строки «Данные о городах — DB-IP» по макету больше
 * нет. Документы — типовой текст-заглушка (LegalDocument). */
type InfoItem = {
  title: string;
  href: string;
};

const infoItems: InfoItem[] = [
  {
    title: 'Политика обработки персональных данных',
    href: ROUTES.profilePrivacy,
  },
  {
    title: 'Пользовательское соглашение',
    href: ROUTES.profileTerms,
  },
  {
    title: 'Публичная оферта',
    href: ROUTES.profileOffer,
  },
];

export default function InfoPage() {
  return (
    <SubScreenShell title="" fallbackHref={ROUTES.profile}>
      <div className="flex flex-col items-center gap-12">
        <RentlyLockup className="h-[183px] w-[254px]" role="img" aria-label="Рентли" />
        <nav aria-label="Правовая информация" className="flex w-full flex-col gap-2">
          {infoItems.map((item) => (
            <NextLink
              key={item.href}
              href={item.href}
              className="flex min-h-[52px] items-center justify-between rounded-card bg-surface-muted px-6 py-4 outline-none focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface"
              aria-label={`Перейти к документу «${item.title}»`}
            >
              <span className="text-base font-medium leading-[18px] text-content">
                {item.title}
              </span>
              {/* Слот шеврона 44 (Icon Button макета): задаёт строке высоту
                  76 = 44 + паддинги 16×2 — все строки карточки одной высоты. */}
              <span className="flex size-11 shrink-0 items-center justify-center">
                <SmallArrowRight className="text-content-tertiary" aria-hidden />
              </span>
            </NextLink>
          ))}
        </nav>
        <div className="flex flex-col items-center gap-5 pb-6 text-center">
          <a
            href={`mailto:${SUPPORT_EMAIL}`}
            className="rounded-card text-xl leading-6 text-primary outline-none focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface"
          >
            {SUPPORT_EMAIL}
          </a>
          <p className="m-0 text-base leading-[18px] text-content-tertiary">
            © Рентли {new Date().getFullYear()}
          </p>
        </div>
      </div>
    </SubScreenShell>
  );
}

import { cookies } from "next/headers";
import { CookieBanner } from "@/components/cookie-banner";
import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";
import {
  COOKIE_CONSENT_ACCEPTED,
  COOKIE_CONSENT_COOKIE,
} from "@/lib/cookie-consent";

export default async function SiteLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  // Согласие на cookie проверяет сервер: баннер не рендерится вовсе,
  // а не прячется гидрацией (нет вспышки при загрузке).
  const cookieConsentGiven =
    (await cookies()).get(COOKIE_CONSENT_COOKIE)?.value ===
    COOKIE_CONSENT_ACCEPTED;

  return (
    <div id="landing-root" className="flex min-h-screen w-full flex-col">
      <SiteHeader />
      <main className="flex-1">{children}</main>
      <SiteFooter />
      {/* Точка гейта аналитики: когда на лендинге появятся скрипты метрик,
          их загрузку разрешать только при cookieConsentGiven. */}
      {!cookieConsentGiven && <CookieBanner />}
    </div>
  );
}

import { Suspense, type ReactNode } from "react";
import Link from "next/link";
import { LandingLink } from "./button";
import { Logo } from "./logo";
import { MobileMenu } from "./mobile-menu";
import { ScrollShadow } from "./scroll-shadow";
import { getMe } from "@/lib/auth";
import { NAV } from "@/lib/nav";

// Действие справа: гость — «Войти» на /login кабинета; авторизованный —
// «Профиль» (макет 2859-4421, аннотация «Когда вошел в профиль»).
// Обычные <a>, не next/link: /login и /profile живут в кабинетном
// приложении за Caddy — нужен полный переход, а не роутер лендинга.
function HeaderAction({ authed }: { authed: boolean }) {
  return authed ? (
    <LandingLink size="md" variant="gray" href="/profile">
      Профиль
    </LandingLink>
  ) : (
    <LandingLink size="md" variant="gray" href="/login">
      Войти
    </LandingLink>
  );
}

// Тень появляется только при скролле (вверху её нет — макет 2814-1164),
// мягкая: 0 4px 20px rgba(0,0,0,.08), плавное появление 300ms.
function DesktopBar({ action }: { action: ReactNode }) {
  return (
    <div className="hidden h-16 grid-cols-[1fr_auto_1fr] items-center rounded-3xl bg-white pl-4 pr-2.5 shadow-[0_4px_20px_rgba(0,0,0,0)] transition-shadow duration-300 group-data-scrolled:shadow-[0_4px_20px_rgba(0,0,0,0.08)] desk:grid">
      <Link
        href="/"
        aria-label="Рентли — на главную"
        className="justify-self-start"
      >
        <Logo className="h-8 w-auto" />
      </Link>
      <nav aria-label="Разделы" className="flex items-center gap-2">
        {NAV.map((item) => (
          <a
            key={item.href}
            href={item.href}
            className="rounded-lg px-3 py-2 text-xs text-ink transition-colors duration-200 outline-none hover:text-primary active:text-primary-active focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary"
          >
            {item.label}
          </a>
        ))}
      </nav>
      <div className="flex justify-end py-2.5 pl-2.5">{action}</div>
    </div>
  );
}

// Обвязка хедера: контейнер + десктоп-бар + мобильное меню. И контент, и
// fallback рендерятся в ней, поэтому геометрия двух состояний совпадает
// структурно, а не по копипасте.
function HeaderShell({ action }: { action: ReactNode }) {
  return (
    <div className="mx-auto max-w-[1520px]">
      <DesktopBar action={action} />
      <div className="desk:hidden">
        <MobileMenu nav={NAV} action={action} />
      </div>
    </div>
  );
}

async function HeaderContent() {
  const me = await getMe();
  return <HeaderShell action={<HeaderAction authed={me !== null} />} />;
}

// Fallback Suspense-дырки: гостевой хедер той же геометрии — авторизованным
// чип «Войти»→«Профиль» меняется без сдвига (обе кнопки 44px, прижаты вправо).
function HeaderFallback() {
  return <HeaderShell action={<HeaderAction authed={false} />} />;
}

export function SiteHeader() {
  return (
    <header className="sticky top-0 z-50 px-2 pt-2">
      <ScrollShadow className="group mx-auto max-w-[1520px]">
        <Suspense fallback={<HeaderFallback />}>
          <HeaderContent />
        </Suspense>
      </ScrollShadow>
    </header>
  );
}

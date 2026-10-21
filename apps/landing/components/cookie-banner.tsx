"use client";

import Link from "next/link";
import { useState } from "react";
import { LandingButton } from "@/components/button";
import {
  COOKIE_CONSENT_ACCEPTED,
  COOKIE_CONSENT_COOKIE,
  COOKIE_CONSENT_MAX_AGE,
} from "@/lib/cookie-consent";

// Баннер согласия на файлы cookie (карточка 3587-88689; раскладки —
// 2967-75646 ПК, 3005-78025 планшет, 3008-79107 мобайл). Показывается,
// пока нет согласия; согласие ставит только явный клик «Хорошо» —
// скрытия по скроллу или клику мимо нет (текст макета «продолжая
// использовать сайт, вы соглашаетесь» — информационное согласие).
// Куку ставим клиентом (Path=/, SameSite=Lax, Secure на https), следующий
// SSR-рендер её уже видит. Когда на лендинге появятся скрипты метрик,
// их загрузку гейтить этим же согласием — сейчас гейтить нечего.
export function CookieBanner() {
  const [hidden, setHidden] = useState(false);

  if (hidden) {
    return null;
  }

  const accept = () => {
    const secure = window.location.protocol === "https:" ? "; Secure" : "";
    document.cookie = `${COOKIE_CONSENT_COOKIE}=${COOKIE_CONSENT_ACCEPTED}; Max-Age=${COOKIE_CONSENT_MAX_AGE}; Path=/; SameSite=Lax${secure}`;
    setHidden(true);
  };

  return (
    // Враппер прижат к краю вьюпорта, карточку от края держит его паддинг
    // 24px: на ПК — правый нижний угол (карточка 700px, кадр 2967-75646),
    // на планшете и мобайле — во всю ширину. z-40: под хедером и оверлеями
    // модалок (z-50) — хедер выше оверлеев не поднимать (решение владельца).
    <div className="fixed inset-x-0 bottom-0 z-40 p-6 desk:left-auto">
      {/* Ярусы карточки: мобайл — стопка (p-4, gap-4, текст 14/18, кадр
          3587-88697); планшет — строка (паддинг 16/16/16/20 — pl-5, кадр
          3587-88704); десктоп — строка 700px (24/24/24/32, gap 24, текст
          18/22, кадр 3587-88691). Радиус 24 (--radius-card) и тень
          --shadow-card — по кадрам раскладок. */}
      <section
        aria-label="Использование файлов cookie"
        className="flex w-full flex-col gap-4 rounded-card bg-white p-4 shadow-card tab:flex-row tab:items-center tab:pl-5 desk:w-[700px] desk:gap-6 desk:py-6 desk:pr-6 desk:pl-8"
      >
        <p className="m-0 flex-1 text-balance text-xs text-ink desk:text-r">
          Мы используем файлы cookie, чтобы сделать работу с сайтом удобнее.
          Продолжая использовать сайт, вы соглашаетесь с нашей{" "}
          <Link href="/privacy" className="text-primary">
            политикой конфиденциальности
          </Link>
        </p>
        {/* Кнопка — инстанс лендинговой системы из макета (LandingButton),
            на мобайле на всю ширину под текстом, на планшете и десктопе —
            справа от текста. Боковой паддинг 28px — решение владельца по
            кадрам баннера (инстансы 117/124px = px-28); important-суффикс
            обязателен: LandingButton склеивает классы без tailwind-merge,
            и px-8 канона в стилях позже px-7 — простой класс бы проиграл. */}
        <LandingButton
          size="lg"
          className="w-full shrink-0 px-7! tab:w-auto"
          onClick={accept}
        >
          Хорошо
        </LandingButton>
      </section>
    </div>
  );
}

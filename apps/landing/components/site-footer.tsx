import Link from "next/link";
import { LandingLink } from "./button";
import { Logo } from "./logo";

// Футер — макет 2814-1150: колонка max-w-1000 с шагом 64px (лого 224×56,
// ряд кнопок gray 64px px-32 radius-16 с зазором 9px, нижняя строка 18/22
// с зазором 24px). Ссылки политик — hover в синий (компонент 2865-5440).
export function SiteFooter() {
  return (
    <footer className="px-5 pb-20 desk:px-20">
      <div className="mx-auto flex w-full max-w-[1000px] flex-col gap-16">
        <Link href="/" aria-label="Рентли — на главную" className="w-fit">
          <Logo className="h-14 w-auto" />
        </Link>
        <div className="flex flex-wrap items-center gap-[9px]">
          {/* TODO(владелец): точный хэндл Telegram-канала — плейсхолдер t.me/rentlee */}
          <LandingLink
            variant="gray"
            href="https://t.me/rentlee"
            target="_blank"
            rel="noopener noreferrer"
          >
            Написать в Telegram
          </LandingLink>
          <LandingLink variant="gray" href="mailto:hello@rentlee.ru">
            hello@rentlee.ru
          </LandingLink>
          <LandingLink
            variant="gray"
            href="mailto:hello@rentlee.ru?subject=Вопрос%20по%20Рентли"
          >
            Задать вопрос
          </LandingLink>
        </div>
        <div className="flex flex-wrap items-center gap-6">
          <span className="text-r text-gray-3">© 2026 Рентли</span>
          <Link
            href="/privacy"
            className="text-r text-ink transition-colors duration-200 hover:text-primary"
          >
            Политика конфиденциальности
          </Link>
          <Link
            href="/terms"
            className="text-r text-ink transition-colors duration-200 hover:text-primary"
          >
            Пользовательское соглашение
          </Link>
        </div>
      </div>
    </footer>
  );
}

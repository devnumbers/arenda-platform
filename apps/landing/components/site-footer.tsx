import Link from "next/link";
import { LandingLink } from "./button";
import { FeedbackModal } from "./feedback-modal";
import { Logo } from "./logo";

// Футер — макеты 2814-1150 (десктоп: колонка max-w-1000 с шагом 64px,
// ряд кнопок gray 64px, нижняя строка с копирайтом ПЕРВЫМ) и 2859:3818 /
// 2826:151662 (планшет/мобайл: поля 32, шаг 32, кнопки столбиком во всю
// ширину с зазором 10, юрблок столбиком 16px: политики, копирайт
// последним, шаг 36, нижний паддинг 36). Ссылки политик — hover в синий
// (компонент 2865-5440). Год копирайта — серверный текущий (макет
// литералом «2026»; главная и юрстраницы рендерятся динамически,
// поэтому год переворачивается сам без пересборки).
export function SiteFooter() {
  const year = new Date().getFullYear();

  return (
    <footer className="px-8 pb-9 desk:px-20 desk:pb-20">
      <div className="mx-auto flex w-full max-w-[1000px] flex-col gap-8 desk:gap-16">
        <Link href="/" aria-label="Рентли — на главную" className="w-fit">
          <Logo className="h-14 w-auto" />
        </Link>
        <div className="flex flex-col gap-2.5 desk:flex-row desk:flex-wrap desk:items-center desk:gap-[9px]">
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
          {/* Открывает модалку «Задать вопрос» (та же, что под вопросами
              FAQ; форма уходит письмом через /api/feedback → бекенд). */}
          <FeedbackModal label="Задать вопрос" variant="gray" />
        </div>
        <div className="flex flex-col gap-3.5 text-s desk:flex-row desk:flex-wrap desk:items-center desk:gap-6 desk:text-r">
          {/* Политики — доска 2865-5440: hover #2b7fff, active #176beb,
              фокус — синий текст; Default 18/22, Small (мобайл) 16/20. */}
          <Link
            href="/privacy"
            className="leading-5 text-ink transition-colors duration-200 hover:text-primary active:text-primary-active focus-visible:text-primary desk:order-2 desk:leading-[22px]"
          >
            Политика конфиденциальности
          </Link>
          <Link
            href="/terms"
            className="leading-5 text-ink transition-colors duration-200 hover:text-primary active:text-primary-active focus-visible:text-primary desk:order-3 desk:leading-[22px]"
          >
            Пользовательское соглашение
          </Link>
          <span className="leading-5 text-gray-3 desk:order-1 desk:leading-[22px]">© {year} Рентли</span>
        </div>
      </div>
    </footer>
  );
}

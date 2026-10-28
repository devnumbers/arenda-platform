"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { Logo } from "./logo";
import type { NavItem } from "@/lib/nav";

// Мобильный/планшетный хедер (<1200px) — макеты 2859-4421 (закрыт) и
// 2862-4943 (открыт): плавающая белая плашка radius-24, пункты меню 52px
// (px-24, radius-12, pressed → surface); страница за меню неподвижна.
//
// Анимация открытия — решение владельца 24.10, перенос из референса
// Figma Make (прототип лендинга, header.tsx): меню — часть той же белой
// карточки, отдельной панели больше нет — карточка «растёт» вниз
// (grid-template-rows 0fr↔1fr, 300ms ease-out), содержимое появляется и
// гаснет одним блоком (opacity 300ms), сепаратор border-t живёт в том же
// гаснущем блоке. Закрытие — то же в обратную сторону: transition
// симметричен, машина фаз closing (таймер среза, зеркальный каскад,
// углы/тень до среза — хореография шторки apple.com от 29.09) снята
// целиком. Бургер морфится в крест морфом трёх линий 16×1.5 с шагом 7
// (геометрия SVG макета 2859-4421): средняя гаснет, крайние съезжаются на
// центральную ось и поворачиваются в крест — 240ms, ease-menu (решение
// владельца 27.10 — возврат морфа 29.09 вместо кроссфейда двух иконок
// из референса Figma Make). Тень карточки — только по скроллу
// (ScrollShadow), как в референсе: открытому меню у верхней позиции тень
// не положена.
//
// Функциональность прежняя: страница под открытым меню блокируется без
// смены overflow html (класс menu-scroll-locked: touch-action/overscroll +
// гвард колеса — смена overflow инвалидирует лэйаут всего документа,
// аудит #1483; клавиатурный скролл фона остаётся — осознанный компромисс);
// Esc закрывает меню; меню, открытое на планшете, при переходе окна в desk
// гаснет мгновенно — иначе лок html переживает уход UI из DOM-видимости
// (бургер и панель уже скрыты desk:hidden, закрыть нечем). Это teardown
// при уходе UI, а не пользовательское закрытие.
//
// visibility в transition-списке свёртывания: дискретное свойство — между
// visible и hidden значение держится visible всю длину перехода, поэтому
// свёртывание доигрывает видимым, а в конце уходит в hidden (пункты
// закрытого меню не табабельны); при открытии видимость возвращается сразу.
// Зеркало --breakpoint-desk (globals.css) — та же строка, что в
// rentals-carousel.tsx: tailwind-классы desk:* из JS не прочитать.
const DESK_QUERY = "(min-width: 1200px)";

export function MobileMenu({
  nav,
  action,
}: {
  nav: NavItem[];
  action: ReactNode;
}) {
  const [open, setOpen] = useState(false);

  // Меню, открытое на планшете, при переходе окна в desk гаснет мгновенно:
  // лок html overflow иначе переживает переход — бургер и панель уже
  // скрыты desk:hidden, и закрыть меню, вернув скролл, нечем.
  useEffect(() => {
    const mq = window.matchMedia(DESK_QUERY);
    const onDesk = () => {
      if (mq.matches) {
        setOpen(false);
      }
    };
    mq.addEventListener("change", onDesk);
    return () => mq.removeEventListener("change", onDesk);
  }, []);

  // Блокировка фона на всё открытое состояние — без смены overflow
  // корневого скроллера: она инвалидирует лэйаут всего документа и стоила
  // старту анимации кадр до 425мс на CPU×4 (аудит #1483). touch-action/
  // overscroll лэйаут не трогают; колесо и трекпад (планшет) гасит
  // слушатель, Esc закрывает меню.
  useEffect(() => {
    if (!open) {
      return;
    }
    const html = document.documentElement;
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpen(false);
      }
    };
    html.classList.add("menu-scroll-locked");
    document.addEventListener("wheel", onWheel, { passive: false });
    document.addEventListener("keydown", onKey);
    return () => {
      html.classList.remove("menu-scroll-locked");
      document.removeEventListener("wheel", onWheel);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div className="rounded-3xl bg-white transition-shadow duration-300 group-data-scrolled:shadow-[0_4px_20px_rgba(0,0,0,0.08)]">
      <div className="flex h-16 items-center pl-4 pr-2.5">
        <Link
          href="/"
          aria-label="Рентли — на главную"
          className="flex min-w-0 flex-1 items-center"
        >
          <Logo className="h-8 w-auto" />
        </Link>
        <div className="flex items-center gap-1 py-2.5 pl-2.5">
          {action}
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            aria-expanded={open}
            aria-controls="mobile-menu"
            aria-label={open ? "Закрыть меню" : "Открыть меню"}
            className="flex h-11 w-11 cursor-pointer items-center justify-center rounded-xl bg-surface transition-colors duration-200 active:duration-75 outline-none hover:bg-surface-hover active:bg-surface-active focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary"
          >
            {/* Морф бургер↔крест (29.09, возвращён решением владельца 27.10):
                три линии 16×1.5 с шагом 7 — средняя гаснет, крайние съезжаются
                на центральную ось и складываются в крест. В Tailwind v4
                translate/rotate — нативные CSS-свойства, список перехода сужен
                до точного набора (аудит анимаций DESIGN.md §8); кривая —
                токен --ease-menu. */}
            <span aria-hidden="true" className="relative block h-6 w-6">
              <span
                className={`absolute left-1 top-[11px] block h-[1.5px] w-4 rounded-full bg-ink transition-[translate,rotate,opacity] duration-[240ms] ease-menu ${
                  open ? "opacity-0" : ""
                }`}
              />
              <span
                className={`absolute left-1 top-[11px] block h-[1.5px] w-4 rounded-full bg-ink transition-[translate,rotate] duration-[240ms] ease-menu ${
                  open ? "translate-y-0 rotate-45" : "-translate-y-[7px]"
                }`}
              />
              <span
                className={`absolute left-1 top-[11px] block h-[1.5px] w-4 rounded-full bg-ink transition-[translate,rotate] duration-[240ms] ease-menu ${
                  open ? "translate-y-0 -rotate-45" : "translate-y-[7px]"
                }`}
              />
            </span>
          </button>
        </div>
      </div>
      <div
        id="mobile-menu"
        className={`grid transition-[grid-template-rows,visibility] duration-300 ease-out ${
          open
            ? "visible [grid-template-rows:1fr]"
            : "invisible [grid-template-rows:0fr]"
        }`}
      >
        <nav className="overflow-hidden" aria-label="Разделы">
          <div
            className={`border-t border-line p-3 transition-opacity duration-300 ${
              open ? "opacity-100" : "opacity-0"
            }`}
          >
            <ul className="flex flex-col">
              {nav.map((item) => (
                <li key={item.href}>
                  <a
                    href={item.href}
                    onClick={() => setOpen(false)}
                    className="flex min-h-[52px] items-center rounded-xl px-6 text-s text-ink transition-colors duration-200 active:duration-75 hover:bg-surface active:bg-surface"
                  >
                    {item.label}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        </nav>
      </div>
    </div>
  );
}

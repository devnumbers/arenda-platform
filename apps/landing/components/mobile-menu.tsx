"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { BurgerIcon, CloseIcon } from "./icons";
import { Logo } from "./logo";
import type { NavItem } from "@/lib/nav";

// Мобильный/планшетный хедер (<1200px) — макеты 2859-4421 (закрыт) и
// 2862-4943 (открыт): плавающая белая плашка radius-24, при открытии меню
// строка хедера получает разделитель border-b #ebebeb, под ним панель
// пунктов 52px (16/20, px-24, radius-12, pressed → surface). Раскрытие —
// плавное (grid-rows).
//
// Панель — absolute-оверлей под плашкой (решение владельца 29.09): страница
// за меню не сдвигается ни на пиксель — раньше раскрытие шло «в потоке» и
// толкало весь контент вниз/вверх на высоту панели.
//
// Форма карточки (скругление низа и тень) живёт в состоянии expanded, а не
// open: оно держится до конца сворачивания — иначе скруглённые нижние углы
// плашки мгновенно возвращаются в момент клика и в стыке с панелью на весь
// кадр анимации проглядывает фон страницы. expanded сбрасывается таймаутом
// на шаг длиннее анимации (transitionend в фоновом табе ненадёжен).
export function MobileMenu({
  nav,
  action,
}: {
  nav: NavItem[];
  action: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const [expanded, setExpanded] = useState(false);

  useEffect(() => {
    if (open) {
      return;
    }
    if (!expanded) {
      return;
    }
    const timer = setTimeout(() => setExpanded(false), 400);
    return () => clearTimeout(timer);
  }, [open, expanded]);

  const toggle = () => {
    if (!open) {
      setExpanded(true);
    }
    setOpen((v) => !v);
  };

  return (
    <div
      className={`relative bg-white transition-shadow duration-300 ${
        expanded
          ? "rounded-t-3xl shadow-[0_4px_20px_rgba(0,0,0,0.08)]"
          : "rounded-3xl shadow-[0_4px_20px_rgba(0,0,0,0)] group-data-scrolled:shadow-[0_4px_20px_rgba(0,0,0,0.08)]"
      }`}
    >
      <div
        className={`flex h-16 items-center border-b pl-4 pr-2.5 transition-colors duration-300 ${
          open ? "border-line" : "border-transparent"
        }`}
      >
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
            onClick={toggle}
            aria-expanded={open}
            aria-controls="mobile-menu"
            aria-label={open ? "Закрыть меню" : "Открыть меню"}
            className="flex h-11 w-11 cursor-pointer items-center justify-center rounded-xl bg-surface transition-colors duration-200 outline-none hover:bg-surface-hover active:bg-surface-active focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary"
          >
            {open ? <CloseIcon /> : <BurgerIcon />}
          </button>
        </div>
      </div>
      <div
        id="mobile-menu"
        className={`absolute inset-x-0 top-full grid rounded-b-3xl bg-white shadow-[0_4px_20px_rgba(0,0,0,0.08)] transition-[grid-template-rows,opacity] duration-300 ease-out ${
          open ? "opacity-100 [grid-template-rows:1fr]" : "opacity-0 [grid-template-rows:0fr]"
        }`}
      >
        <nav className="overflow-hidden" aria-label="Разделы">
          <ul className="p-3">
            {nav.map((item) => (
              <li key={item.href}>
                <a
                  href={item.href}
                  onClick={() => setOpen(false)}
                  className="flex min-h-[52px] items-center rounded-xl px-6 text-s text-ink transition-colors duration-200 hover:bg-surface active:bg-surface"
                >
                  {item.label}
                </a>
              </li>
            ))}
          </ul>
        </nav>
      </div>
    </div>
  );
}

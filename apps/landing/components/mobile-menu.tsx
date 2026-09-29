"use client";

import { useEffect, useState, useSyncExternalStore, type ReactNode } from "react";
import Link from "next/link";
import { Logo } from "./logo";
import type { NavItem } from "@/lib/nav";

// Мобильный/планшетный хедер (<1200px) — макеты 2859-4421 (закрыт) и
// 2862-4943 (открыт): плавающая белая плашка radius-24, при открытии под
// строкой хедера панель-оверлей с пунктами 52px (16/20, px-24, radius-12,
// pressed → surface); страница за меню неподвижна.
//
// Анимация — по приёмам apple.com (замер их globalnav 29.09, решение
// владельца): одна кривая на всё cubic-bezier(0.4, 0, 0.6, 1); открытие —
// панель-шторка 400мс (grid-rows) и каскад пунктов — фейд 240мс с подъёмом
// 10px, шаг 45мс; закрытие асимметричное, как у них: панель гаснет на месте
// (те же 400мс, высота не схлопывается) и срезается одним кадром — обратной
// анимации высоты нет. Морф иконки: средняя линия гаснет, крайние съезжаются
// в крест (240мс). Страница под открытым меню блокируется от скролла
// (overflow hidden на html + scrollbar-gutter stable — на классических
// скроллбарах без сдвига контента). prefers-reduced-motion — без анимаций и
// без каскадных задержек.
//
// Машина состояний: open («раскрыто»; true и в фазе гаснения — пункты и
// высота держатся, гаснет только панель), fading (гаснение на месте),
// expanded (форма карточки — скругление низа и тень — до конца закрытия).
// Срез после гаснения — таймаутом на шаг длиннее фейда: transitionend в
// фоновом табе ненадёжен.
const EASE = "ease-[cubic-bezier(0.4,0,0.6,1)]";
const FADE_CUT_MS = 440;

export function MobileMenu({
  nav,
  action,
}: {
  nav: NavItem[];
  action: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const [fading, setFading] = useState(false);
  const [expanded, setExpanded] = useState(false);
  // Канон клиент-only значения (гидратация без React #418): сервер снапшот —
  // false, клиентский подписывается на живое переключение media query.
  const reducedMotion = useSyncExternalStore(
    (onChange) => {
      const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
      mq.addEventListener("change", onChange);
      return () => mq.removeEventListener("change", onChange);
    },
    () => window.matchMedia("(prefers-reduced-motion: reduce)").matches,
    () => false,
  );

  useEffect(() => {
    if (!fading) {
      return;
    }
    const timer = setTimeout(() => {
      setOpen(false);
      setFading(false);
      setExpanded(false);
    }, FADE_CUT_MS);
    return () => clearTimeout(timer);
  }, [fading]);

  // Лок скролла на всё открытое состояние (включая гаснение): overflow
  // правим напрямую (внешняя система), gutter резервирует полосу классического
  // скроллбара, чтобы блокировка не двигала контент.
  useEffect(() => {
    if (!open) {
      return;
    }
    const html = document.documentElement;
    html.style.overflow = "hidden";
    html.style.scrollbarGutter = "stable";
    return () => {
      html.style.overflow = "";
      html.style.scrollbarGutter = "";
    };
  }, [open]);

  const startClose = () => {
    if (open && !fading) {
      setFading(true);
    }
  };

  const toggle = () => {
    if (fading) {
      setFading(false);
      return;
    }
    if (open) {
      setFading(true);
      return;
    }
    setExpanded(true);
    setOpen(true);
  };

  const shown = open && !fading;

  return (
    <div
      className={`relative bg-white transition-shadow duration-300 ${
        expanded
          ? "rounded-t-3xl shadow-[0_4px_20px_rgba(0,0,0,0.08)]"
          : "rounded-3xl shadow-[0_4px_20px_rgba(0,0,0,0)] group-data-scrolled:shadow-[0_4px_20px_rgba(0,0,0,0.08)]"
      }`}
    >
      <div
        className={`flex h-16 items-center border-b pl-4 pr-2.5 transition-colors duration-[240ms] ${EASE} ${
          shown ? "border-line" : "border-transparent"
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
            aria-expanded={shown}
            aria-controls="mobile-menu"
            aria-label={shown ? "Закрыть меню" : "Открыть меню"}
            className="flex h-11 w-11 cursor-pointer items-center justify-center rounded-xl bg-surface transition-colors duration-200 outline-none hover:bg-surface-hover active:bg-surface-active focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary"
          >
            {/* Бургер 2859-4421 (3 линии 16×1.5 с шагом 7) морфится в крест:
                средняя гаснет, крайние съезжаются на центральную ось. */}
            <span aria-hidden="true" className="relative block h-6 w-6">
              <span
                className={`absolute left-1 top-[11px] block h-[1.5px] w-4 rounded-full bg-ink transition-[translate,rotate,opacity] duration-[240ms] ${EASE} motion-reduce:transition-none ${
                  shown ? "opacity-0" : ""
                }`}
              />
              <span
                className={`absolute left-1 top-[11px] block h-[1.5px] w-4 rounded-full bg-ink transition-[translate,rotate] duration-[240ms] ${EASE} motion-reduce:transition-none ${
                  shown ? "translate-y-0 rotate-45" : "-translate-y-[7px]"
                }`}
              />
              <span
                className={`absolute left-1 top-[11px] block h-[1.5px] w-4 rounded-full bg-ink transition-[translate,rotate] duration-[240ms] ${EASE} motion-reduce:transition-none ${
                  shown ? "translate-y-0 -rotate-45" : "translate-y-[7px]"
                }`}
              />
            </span>
          </button>
        </div>
      </div>
      <div
        id="mobile-menu"
        className={`absolute inset-x-0 top-full grid rounded-b-3xl bg-white shadow-[0_4px_20px_rgba(0,0,0,0.08)] transition-[grid-template-rows,opacity] duration-[400ms] ${EASE} motion-reduce:transition-none ${
          fading ? "opacity-0" : "opacity-100"
        } ${open ? "visible [grid-template-rows:1fr]" : "invisible [grid-template-rows:0fr]"}`}
      >
        <nav className="overflow-hidden" aria-label="Разделы">
          <ul className="p-3">
            {nav.map((item, i) => (
              <li key={item.href}>
                <a
                  href={item.href}
                  onClick={startClose}
                  style={{
                    transitionDelay:
                      shown && !reducedMotion ? `${i * 45}ms` : "0ms",
                  }}
                  className={`flex min-h-[52px] items-center rounded-xl px-6 text-s text-ink transition-[opacity,translate] duration-[240ms] ${EASE} motion-reduce:transition-none hover:bg-surface active:bg-surface ${
                    open
                      ? "translate-y-0 opacity-100"
                      : "translate-y-2.5 opacity-0"
                  }`}
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

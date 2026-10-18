"use client";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import { Logo } from "./logo";
import type { NavItem } from "@/lib/nav";

// Мобильный/планшетный хедер (<1200px) — макеты 2859-4421 (закрыт) и
// 2862-4943 (открыт): плавающая белая плашка radius-24, при открытии под
// строкой хедера панель-оверлей с пунктами 52px (16/20, px-24, radius-12,
// pressed → surface); страница за меню неподвижна.
//
// Анимация — по приёмам apple.com (замер их globalnav 29.09): одна кривая на
// всё cubic-bezier(0.4, 0, 0.6, 1); открытие — панель-шторка 400мс
// (grid-rows) и каскад пунктов — фейд 240мс с подъёмом 10px, шаг 45мс.
// Закрытие — ЗЕРКАЛО открытия (решение владельца 29.09 после живого
// просмотра Apple-асимметрии «гаснет на месте + срез»): та же шторка вниз и
// обратный каскад пунктов (нижние гаснут первыми — поднимающийся край шторки
// их догоняет). Форма карточки (прямые нижние углы + тень) держится до
// самого среза — скругление возвращается в конце, как было: живой радиус при
// висящей панели давал клинья в стыке (решение владельца 29.09 позднее —
// откат эксперимента со скруглением на старте закрытия).
// Морф иконки: средняя линия гаснет, крайние съезжаются в крест (240мс).
// Страница под открытым меню блокируется от скролла без смены overflow
// html (класс menu-scroll-locked: touch-action/overscroll + гвард колеса
// — смена overflow инвалидирует лэйаут всего документа, аудит #1483).
// Esc закрывает меню.
//
// Машина состояний: open («раскрыто» — высота, каскад), closing (свёртывание:
// open уже false, панель едет вниз до среза; срез — таймаутом на шаг длиннее
// свёртывания: transitionend в фоновом табе ненадёжен). Форма карточки —
// прямые нижние углы и тень — живёт на open||closing до самого среза.
// Повторное открытие посреди closing продолжает высоту с текущего кадра и
// перезапускает каскад.
const EASE = "ease-menu";
const CLOSE_CUT_MS = 440;
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
  const [closing, setClosing] = useState(false);

  useEffect(() => {
    if (!closing) {
      return;
    }
    const timer = setTimeout(() => setClosing(false), CLOSE_CUT_MS);
    return () => clearTimeout(timer);
  }, [closing]);

  // Меню, открытое на планшете, при переходе окна в desk гаснет мгновенно —
  // иначе лок html overflow:hidden переживает переход: бургер и панель уже
  // скрыты desk:hidden, и закрыть меню, вернув скролл, нечем. Это teardown
  // при уходе UI из DOM-видимости, а не пользовательское закрытие — фаза
  // closing не заводится, таймер среза отменяет свой cleanup.
  useEffect(() => {
    const mq = window.matchMedia(DESK_QUERY);
    const onDesk = () => {
      if (mq.matches) {
        setOpen(false);
        setClosing(false);
      }
    };
    mq.addEventListener("change", onDesk);
    return () => mq.removeEventListener("change", onDesk);
  }, []);

  const startClose = useCallback(() => {
    if (open && !closing) {
      setOpen(false);
      setClosing(true);
    }
  }, [open, closing]);
  // Мост в эффект лока: листенеры зовут свежий startClose, не включая
  // его в зависимости (канон goToRef в testimonials-carousel).
  const startCloseRef = useRef<() => void>(() => {});
  useEffect(() => {
    startCloseRef.current = startClose;
  }, [startClose]);

  // Блокировка фона на всё открытое состояние (включая свёртывание) —
  // без смены overflow корневого скроллера: она инвалидирует лэйаут
  // всего документа и стоила старту шторки кадр до 425мс на CPU×4
  // (аудит #1483). touch-action/overscroll лэйаут не трогают; колесо и
  // трекпад (планшет) гасит слушатель, Esc закрывает меню. Клавиатурный
  // скролл фона (space/стрелки) остаётся возможным — осознанный
  // компромисс.
  useEffect(() => {
    if (!open && !closing) {
      return;
    }
    const html = document.documentElement;
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        startCloseRef.current();
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
  }, [open, closing]);

  const toggle = () => {
    if (closing) {
      setClosing(false);
      setOpen(true);
      return;
    }
    if (open) {
      startClose();
      return;
    }
    setOpen(true);
  };

  return (
    <div
      className={`relative bg-white transition-shadow duration-300 ${
        open || closing
          ? "rounded-t-3xl shadow-[0_4px_20px_rgba(0,0,0,0.08)]"
          : "rounded-3xl shadow-[0_4px_20px_rgba(0,0,0,0)] group-data-scrolled:shadow-[0_4px_20px_rgba(0,0,0,0.08)]"
      }`}
    >
      <div
        className={`flex h-16 items-center border-b pl-4 pr-2.5 transition-colors duration-[240ms] ${EASE} ${
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
            className="flex h-11 w-11 cursor-pointer items-center justify-center rounded-xl bg-surface transition-colors duration-200 active:duration-75 outline-none hover:bg-surface-hover active:bg-surface-active focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary"
          >
            {/* Бургер 2859-4421 (3 линии 16×1.5 с шагом 7) морфится в крест:
                средняя гаснет, крайние съезжаются на центральную ось. */}
            <span aria-hidden="true" className="relative block h-6 w-6">
              <span
                className={`absolute left-1 top-[11px] block h-[1.5px] w-4 rounded-full bg-ink transition-[translate,rotate,opacity] duration-[240ms] ${EASE} ${
                  open ? "opacity-0" : ""
                }`}
              />
              <span
                className={`absolute left-1 top-[11px] block h-[1.5px] w-4 rounded-full bg-ink transition-[translate,rotate] duration-[240ms] ${EASE} ${
                  open ? "translate-y-0 rotate-45" : "-translate-y-[7px]"
                }`}
              />
              <span
                className={`absolute left-1 top-[11px] block h-[1.5px] w-4 rounded-full bg-ink transition-[translate,rotate] duration-[240ms] ${EASE} ${
                  open ? "translate-y-0 -rotate-45" : "translate-y-[7px]"
                }`}
              />
            </span>
          </button>
        </div>
      </div>
      <div
        id="mobile-menu"
        className={`absolute inset-x-0 top-full grid rounded-b-3xl bg-white shadow-[0_4px_20px_rgba(0,0,0,0.08)] transition-[grid-template-rows] duration-[400ms] ${EASE} ${
          open || closing ? "visible" : "invisible"
        } ${open ? "[grid-template-rows:1fr]" : "[grid-template-rows:0fr]"}`}
      >
        <nav className="overflow-hidden" aria-label="Разделы">
          <ul className="p-3">
            {nav.map((item, i) => (
              <li key={item.href}>
                <a
                  href={item.href}
                  onClick={startClose}
                  style={{
                    transitionDelay: open
                      ? `${i * 45}ms`
                      : closing
                        ? `${(nav.length - 1 - i) * 45}ms`
                        : "0ms",
                  }}
                  className={`flex min-h-[52px] items-center rounded-xl px-6 text-s text-ink transition-[opacity,translate] duration-[240ms] ${EASE} hover:bg-surface active:bg-surface ${
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

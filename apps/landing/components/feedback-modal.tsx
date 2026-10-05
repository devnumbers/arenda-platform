"use client";

import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { createPortal } from "react-dom";
import { LandingButton } from "./button";

type Phase = "form" | "sending" | "sent";

// Модалка «Задать вопрос» (карта #1010, кадры 3012-82067 — форма и
// 3013-82245 — успех): белая панель r40 p-24, максимум 400px с паддингом
// (решение владельца), оверлей rgba(23,26,28,.5); «Свяжитесь с нами»
// 22/26 semibold + подзаголовок 16/18 серым; поля «Электронная почта» /
// «Сообщение» — подпись 16/18 Medium, бокс surface r16 высотой 56/110,
// текст 16/18; успех — «Сообщение отправлено» + «Хорошо» по центру, крестик
// скрыт. Отправка — на свой /api/feedback (прокси в бекенд, почту шлёт он);
// honeypot «website» невидим для людей и читается из FormData. Закрытие —
// крестик, Esc, клик по оверлею (в успехе — теми же путями и «Хорошо»);
// фокус при открытии — в поле почты, при закрытии — назад на триггер,
// Tab зациклен внутри панели. Триггер — сама кнопка: FAQ держит primary sm,
// футер — gray lg; каждый экземпляр независим, панель рендерится в portal
// (FAQ-триггер живёт внутри Reveal с transform — fixed внутри него сломался бы).
const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const EMAIL_MAX = 254;
const MESSAGE_MAX = 1000;

export function FeedbackModal({
  label,
  variant = "primary",
  size = "lg",
}: {
  label: string;
  variant?: "primary" | "gray";
  size?: "sm" | "lg";
}) {
  const [open, setOpen] = useState(false);
  const [phase, setPhase] = useState<Phase>("form");
  const [error, setError] = useState<string | null>(null);
  const [email, setEmail] = useState("");
  const [message, setMessage] = useState("");
  const panelRef = useRef<HTMLDivElement>(null);
  const emailInputRef = useRef<HTMLInputElement>(null);
  const lastFocusedRef = useRef<HTMLElement | null>(null);

  const close = useCallback(() => {
    setOpen(false);
    setPhase("form");
    setError(null);
    setEmail("");
    setMessage("");
    lastFocusedRef.current?.focus();
  }, []);

  const openModal = () => {
    lastFocusedRef.current = document.activeElement as HTMLElement | null;
    setOpen(true);
  };

  // Лок скролла страницы — канон mobile-menu: overflow напрямую на html,
  // gutter резервирует полосу скроллбара, чтобы блокировка не двигала контент.
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

  useEffect(() => {
    if (!open) {
      return;
    }
    emailInputRef.current?.focus();
    // Esc закрывает; Tab зациклен внутри панели (фокус-трап) — слушателем
    // на document, не реквизитом на div (jsx-a11y запрещает хендлеры на
    // неинтерактивных элементах); honeypot с tabindex -1 в выборку не попадает.
    function onKey(e: globalThis.KeyboardEvent) {
      if (e.key === "Escape") {
        close();
        return;
      }
      if (e.key !== "Tab" || !panelRef.current) {
        return;
      }
      const focusables = panelRef.current.querySelectorAll<HTMLElement>(
        'button:not([tabindex="-1"]), input:not([tabindex="-1"]), textarea:not([tabindex="-1"])',
      );
      if (focusables.length === 0) {
        return;
      }
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      if (!first || !last) {
        return;
      }
      const active = document.activeElement;
      const inside = panelRef.current.contains(active);
      if (e.shiftKey && (active === first || !inside)) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && (active === last || !inside)) {
        e.preventDefault();
        first.focus();
      }
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, close]);

  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (phase !== "form") {
      return;
    }
    setError(null);
    if (!EMAIL_RE.test(email.trim()) || email.trim().length > EMAIL_MAX) {
      setError("Проверьте адрес электронной почты");
      return;
    }
    if (message.trim() === "" || message.trim().length > MESSAGE_MAX) {
      setError("Напишите ваш вопрос");
      return;
    }
    setPhase("sending");
    try {
      const honeypot = new FormData(e.currentTarget).get("website");
      const res = await fetch("/api/feedback", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email: email.trim(),
          message: message.trim(),
          website: typeof honeypot === "string" ? honeypot : "",
        }),
      });
      if (res.status === 204) {
        setPhase("sent");
        return;
      }
      setPhase("form");
      setError(
        res.status === 429
          ? "Слишком много попыток. Попробуйте позже"
          : "Не отправилось. Попробуйте ещё раз",
      );
    } catch {
      setPhase("form");
      setError("Не отправилось. Попробуйте ещё раз");
    }
  };

  return (
    <>
      <LandingButton variant={variant} size={size} onClick={openModal}>
        {label}
      </LandingButton>
      {open &&
        createPortal(
          <div
            className="fixed inset-0 z-50 overflow-y-auto"
            role="dialog"
            aria-modal="true"
            aria-labelledby="feedback-modal-title"
          >
            <div className="absolute inset-0 bg-ink/50" onClick={close} aria-hidden="true" />
            <div className="relative flex min-h-full items-center justify-center p-6">
              <div
                ref={panelRef}
                className="relative w-full max-w-[400px] rounded-[40px] bg-white p-6"
              >
                {phase !== "sent" && (
                  <button
                    type="button"
                    onClick={close}
                    aria-label="Закрыть"
                    className="absolute right-3.5 top-3.5 flex size-11 cursor-pointer items-center justify-center rounded-full text-ink transition-colors duration-200 outline-none hover:bg-surface focus-visible:ring-2 focus-visible:ring-primary/40"
                  >
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                      <path
                        d="M6.5 6.5L17.5 17.5M17.5 6.5L6.5 17.5"
                        stroke="currentColor"
                        strokeWidth="1.5"
                        strokeLinecap="round"
                      />
                    </svg>
                  </button>
                )}
                {phase === "sent" ? (
                  <div className="flex flex-col items-stretch gap-8">
                    <p
                      id="feedback-modal-title"
                      className="text-center text-l font-semibold text-ink"
                    >
                      Сообщение отправлено
                    </p>
                    <LandingButton size="sm" className="w-full" onClick={close}>
                      Хорошо
                    </LandingButton>
                  </div>
                ) : (
                  <form
                    className="flex flex-col items-stretch gap-8"
                    onSubmit={(e) => {
                      void submit(e);
                    }}
                    noValidate
                  >
                    <div className="flex flex-col gap-2">
                      <p id="feedback-modal-title" className="text-l font-semibold text-ink">
                        Свяжитесь с нами
                      </p>
                      <p className="text-[16px] leading-[18px] text-gray-2">
                        Напишите нам, если у вас есть вопросы
                      </p>
                    </div>
                    <div className="flex flex-col gap-6">
                      <div className="flex flex-col gap-2">
                        <label
                          htmlFor="feedback-email"
                          className="text-[16px] leading-[18px] font-medium text-ink"
                        >
                          Электронная почта
                        </label>
                        <input
                          ref={emailInputRef}
                          id="feedback-email"
                          name="email"
                          type="email"
                          autoComplete="email"
                          maxLength={EMAIL_MAX}
                          value={email}
                          onChange={(e) => setEmail(e.target.value)}
                          className="h-14 rounded-2xl bg-surface px-[18px] text-[16px] leading-[18px] text-ink outline-none focus:ring-2 focus:ring-inset focus:ring-primary/40"
                        />
                      </div>
                      <div className="flex flex-col gap-2">
                        <label
                          htmlFor="feedback-message"
                          className="text-[16px] leading-[18px] font-medium text-ink"
                        >
                          Сообщение
                        </label>
                        <textarea
                          id="feedback-message"
                          name="message"
                          maxLength={MESSAGE_MAX}
                          value={message}
                          onChange={(e) => setMessage(e.target.value)}
                          className="min-h-[110px] resize-none rounded-2xl bg-surface px-[18px] py-[10px] text-[16px] leading-[18px] text-ink outline-none focus:ring-2 focus:ring-inset focus:ring-primary/40"
                        />
                      </div>
                      {/* Honeypot: люди поля не видят, боты заполняют —
                          прокси отвечает успехом, бекенд не дёргается. */}
                      <input
                        type="text"
                        name="website"
                        hidden
                        tabIndex={-1}
                        autoComplete="off"
                        aria-hidden="true"
                      />
                    </div>
                    <div className="flex flex-col items-stretch gap-3">
                      {error && (
                        <p role="alert" className="text-center text-s leading-5 text-[#fb2c36]">
                          {error}
                        </p>
                      )}
                      <LandingButton
                        size="sm"
                        className="w-full disabled:bg-primary-disabled"
                        type="submit"
                        disabled={phase === "sending"}
                        aria-busy={phase === "sending"}
                      >
                        Отправить
                      </LandingButton>
                    </div>
                  </form>
                )}
              </div>
            </div>
          </div>,
          document.body,
        )}
    </>
  );
}

"use client";

import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { LandingButton } from "@/components/button";
import { IconButton } from "@/components/ui/icon-button";
import { Cancel } from "@/components/ui/icons";
import { Modal, ModalContent, useIsDesktop } from "@/components/ui/modal";
import { TextField } from "@/components/ui/text-field";
import { Textarea } from "@/components/ui/text-area";

type Phase = "form" | "sending" | "sent";

// Модалка «Задать вопрос» (карта #1010, кадры 3012-82067 — форма и
// 3013-82245 — успех) на перенесённых ui-компонентах фронта (решение
// владельца: «возьми готовые ui компоненты из фронта — так правильнее»):
// шелл Modal = Radix Dialog ≥768 (карточка: вход 400мс / выход 220мс) и
// vaul-шит <768 (свайп вниз, ручка; вход/выход анимирует vaul) — плавность,
// закрытие по фону/Esc/свайпу, фокус-трап и лок скролла библиотечные;
// поля — TextField/Textarea (тот самый «Input Field» 948:46646 из кадра:
// подпись 16/18 Medium, бокс surface-muted r16 h-56, hover-обводка, без
// фокус-кольца — канон продукта; Textarea автoрастёт от 92px — канон
// #505 вместо зафиксированных 110px кадра). Крестик — только в карточке
// (в шите закрывают свайп/оверлей — канон шелла). Кнопки — лендинговая
// LandingButton (инстанс лендинговой же системы в макете: 56px, px-32,
// 16/20). max-w 400 с паддингом — решение владельца (как SupportModal
// фронта). Форма: «Свяжитесь с нами» 22/26 + подзаголовок 16/18; оба поля
// обязательны (email по regex ≤254, сообщение ≤1000 — maxLength-атрибуты
// не вешаем: у дизайн-полей они рисуют счётчик, которого в кадре нет —
// длину сторожат клиентская валидация + прокси + бекенд). Ошибка отправки
// — инлайн text-error под кнопкой (решение владельца), поля сохраняются.
// Успех — «Сообщение отправлено» + «Хорошо» по центру. Отправка на свой
// /api/feedback (прокси в бекенд, почту шлёт он); honeypot «website»
// невидим для людей и читается из FormData. Триггеров два (FAQ и футер) —
// каждый экземпляр независим; закрытие сбрасывает форму.
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
  const [closing, setClosing] = useState(false);
  const [phase, setPhase] = useState<Phase>("form");
  const [error, setError] = useState<string | null>(null);
  const [email, setEmail] = useState("");
  const [message, setMessage] = useState("");
  const closeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastFocusedRef = useRef<HTMLElement | null>(null);
  const isDesktop = useIsDesktop();

  useEffect(() => () => {
    if (closeTimerRef.current) clearTimeout(closeTimerRef.current);
  }, []);

  // Открытие всегда с чистой формой (сброс живёт здесь, а не при закрытии:
  // затухающая панель не должна пустеть на глазах); повторный клик по
  // триггеру во время 250мс-фазы закрытия отменяет её и открывает заново.
  const openModal = useCallback(() => {
    if (closeTimerRef.current) {
      clearTimeout(closeTimerRef.current);
      closeTimerRef.current = null;
    }
    setPhase("form");
    setError(null);
    setEmail("");
    setMessage("");
    setClosing(false);
    setOpen(true);
  }, []);

  // Закрытие: на карточке (≥768) держим open 250мс с closing — классы
  // выхода применяются детерминированно (Presence Radix на React 19.2.4
  // иначе размонтирует мгновенно, см. modal.tsx); на шите открытый/vaul
  // сам доигрывает slideToBottom + fadeOut.
  const close = useCallback(() => {
    if (isDesktop) {
      setClosing(true);
      closeTimerRef.current = setTimeout(() => {
        setOpen(false);
        setClosing(false);
      }, 250);
      return;
    }
    setOpen(false);
  }, [isDesktop]);

  const onOpenChange = useCallback(
    (next: boolean) => {
      if (next) {
        openModal();
        return;
      }
      close();
    },
    [openModal, close],
  );

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
    <Modal open={open} onOpenChange={onOpenChange}>
      {/* Триггер запоминаем из события (e.currentTarget), а не по
          document.activeElement: на macOS клик мышью кнопку не фокусирует.
          focus() возвращает фокус на триггер на всех платформах — канон
          DialogTrigger фронта. */}
      <LandingButton
        variant={variant}
        size={size}
        onClick={(e) => {
          lastFocusedRef.current = e.currentTarget;
          openModal();
        }}
      >
        {label}
      </LandingButton>
      <ModalContent
        title="Задать вопрос"
        titleSrOnly
        className="max-w-[400px]"
        closing={isDesktop && closing}
        onCloseAutoFocus={(e) => {
          // Radix при unmount тянет фокус в «элемент до диалога» (на маке —
          // body: клик мышью кнопку не фокусирует). Возвращаем на триггер —
          // как DialogTrigger фронта.
          e.preventDefault();
          lastFocusedRef.current?.focus();
        }}
      >
        <div className="flex flex-col gap-8">
          <div className="flex items-start justify-between gap-4">
            <div className="flex flex-col gap-2">
              {/* A11y-имя диалога даёт sr-only DialogTitle шелла; видимый
                   заголовок — по кадру 3012-82067, от скринридеров скрыт. */}
              <h2 aria-hidden className="m-0 text-l font-semibold text-ink">
                Свяжитесь с нами
              </h2>
              <p className="m-0 text-[16px] leading-[18px] text-content-secondary">
                Напишите нам, если у вас есть вопросы
              </p>
            </div>
            {/* Крестик — канон шелла: только в карточке ≥768; в шите
                закрывают свайп вниз или тап по оверлею. */}
            <IconButton
              icon={<Cancel />}
              label="Закрыть"
              className="hidden min-[768px]:flex"
              onClick={close}
            />
          </div>
          {phase === "sent" ? (
            <div className="flex flex-col items-stretch gap-8">
              <p className="m-0 text-center text-l font-semibold text-ink">Сообщение отправлено</p>
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
              <div className="flex flex-col gap-6">
                <TextField
                  title="Электронная почта"
                  type="email"
                  autoComplete="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
                <Textarea
                  title="Сообщение"
                  value={message}
                  onChange={(e) => setMessage(e.target.value)}
                />
                {/* Honeypot: люди поля не видят, боты заполняют — прокси
                    отвечает успехом, бекенд не дёргается. */}
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
                  <p role="alert" className="m-0 text-center text-s leading-5 text-error">
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
      </ModalContent>
    </Modal>
  );
}

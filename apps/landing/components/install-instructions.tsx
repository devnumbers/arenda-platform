"use client";

import { useEffect, useRef, useState, type JSX } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import Image from "next/image";
import type { StaticImageData } from "next/image";
import { LandingButton } from "@/components/button";
import { cn } from "@/lib/cn";
import { IconButton } from "@/components/ui/icon-button";
import { Cancel } from "@/components/ui/icons";
import iconBrowserChrome from "@/assets/icons/icon-browser-chrome.webp";
import iconBrowserYandex from "@/assets/icons/icon-browser-yandex.webp";
import iconBrowserSafari from "@/assets/icons/icon-browser-safari.webp";

// Модалка «Установка для <браузера>» — кадры 3350-160996 (Chrome),
// 3406-78074 (Яндекс Браузера), 3406-78168 (Safari) на общем компоненте
// Modal (936:33834, вариант Popup): оверлей rgba(23,26,28,.5), белая
// карточка r40 p-24 ПО ЦЕНТРУ на любой ширине окна (дизайнер выбрал
// попап, не нижний шит — контент короткий, скроллить и тянуть нечего),
// крестик в правом верхнем углу. Контент: иконка 48 (у Chrome)/40
// (Яндекс, Safari) + заголовок 22/26 SemiBold, секции с шагом 24 —
// серая метка 16/18 Medium (#6F787C) и нумерованные шаги 16/18 Medium,
// кнопка «Понятно» Blue Small во всю ширину (p 0/28 по кадру).
//
// Шелл — Radix Dialog напрямую, не components/ui/modal (тот на <768
// рисует vaul-шит по канону продукта, а здесь по кадру попап на всех
// ширинах; править перенос из фронта = drift). Выход — управляемая фаза
// closing (~250мс) как в feedback-modal: Radix Presence на React 19
// размонтирует контент мгновенно и сносит exit-анимацию; машина фазы
// (открытие/закрытие/отмена перезакрытием) живёт в секции contact —
// тот же владелец состояния, что у кнопок, все манипуляции в
// обработчиках событий, модалка чисто предъявительская.
export type BrowserId = "chrome" | "yandex" | "safari";

type InstructionSection = {
  label?: string;
  steps: string[];
};

type Browser = {
  label: string;
  icon: StaticImageData;
  iconClassName: string;
  title: string;
  sections: InstructionSection[];
};

export const BROWSERS: Record<BrowserId, Browser> = {
  chrome: {
    label: "Google Chrome",
    icon: iconBrowserChrome,
    iconClassName: "size-12",
    title: "Установка для Google Chrome",
    sections: [
      {
        label: "Первый способ:",
        steps: ["Нажмите на значок «Установить» в адресной строке"],
      },
      {
        label: "Второй способ:",
        steps: [
          "В правом верхнем углу нажмите на значок с тремя точками",
          "Перейдите в раздел «Транслировать, сохранить, поделиться»",
          "Нажмите «Установить приложение “Рентли”»",
        ],
      },
    ],
  },
  yandex: {
    label: "Яндекс Браузера",
    icon: iconBrowserYandex,
    iconClassName: "size-10",
    title: "Установка для Яндекс Браузера",
    sections: [
      {
        steps: [
          "Откройте меню браузера в правом верхнем углу",
          "Перейдите в раздел «Дополнительно»",
          "Выберите пункт «Установить как приложение»",
        ],
      },
    ],
  },
  safari: {
    label: "Safari",
    icon: iconBrowserSafari,
    iconClassName: "size-10",
    title: "Установка для Safari",
    sections: [
      {
        label: "На iOS",
        steps: [
          "В правом верхнем углу нажмите на кнопку «Поделиться»",
          "Нажмите «Показать больше»",
          "Нажмите «Добавить на экран Домой»",
        ],
      },
      {
        // В кадре Safari блок «На macOS» — заглушка-дубль (два одинаковых
        // пункта с номером 1): шаги дописаны по реальному флоу Safari —
        // «Добавить в Dock» (на согласование владельцу).
        label: "На macOS",
        steps: [
          "На панели инструментов нажмите кнопку «Поделиться»",
          "Выберите «Добавить в Dock»",
        ],
      },
    ],
  },
};

const CLOSE_MS = 250;

export function InstallModal({
  browser,
  closing,
  onClose,
}: {
  browser: BrowserId;
  closing: boolean;
  onClose: () => void;
}): JSX.Element {
  const current = BROWSERS[browser];

  return (
    <Dialog.Root
      open
      onOpenChange={(next) => {
        if (!next) {
          onClose();
        }
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay
          className={cn(
            "fixed inset-0 z-50 bg-overlay",
            closing ? "dl-overlay-out" : "dl-overlay-in",
          )}
        />
        <div className="fixed inset-0 z-50 flex items-center justify-center p-6">
          <Dialog.Content
            aria-describedby={undefined}
            className={cn(
              "relative flex max-h-[92dvh] w-full max-w-[400px] flex-col gap-6 overflow-y-auto overscroll-y-contain rounded-[40px] bg-white p-6 font-sans outline-none",
              closing ? "dl-card-out" : "dl-card-in",
            )}
          >
            <div className="flex flex-col gap-4">
              <div className="flex flex-col gap-3 pr-12">
                <Image
                  src={current.icon}
                  alt=""
                  aria-hidden
                  width={current.icon.width}
                  height={current.icon.height}
                  className={current.iconClassName}
                />
                <Dialog.Title className="text-l font-semibold leading-[26px]">
                  {current.title}
                </Dialog.Title>
              </div>
              <div className="flex flex-col gap-6">
                {current.sections.map((section, index) => (
                  <div
                    // Секции статичны, ключом служит их содержимое.
                    key={section.label ?? `section-${index}`}
                    className="flex flex-col gap-3"
                  >
                    {section.label !== undefined && (
                      <p className="text-s font-medium leading-[18px] text-content-secondary">
                        {section.label}
                      </p>
                    )}
                    <ol className="flex list-decimal flex-col gap-3 pl-5 marker:font-medium">
                      {section.steps.map((step) => (
                        <li key={step} className="text-s font-medium leading-[18px] text-ink">
                          {step}
                        </li>
                      ))}
                    </ol>
                  </div>
                ))}
              </div>
            </div>
            <Dialog.Close asChild>
              <LandingButton size="sm" className="w-full px-7">
                Понятно
              </LandingButton>
            </Dialog.Close>
            <Dialog.Close asChild>
              <IconButton
                icon={<Cancel />}
                label="Закрыть"
                className="absolute right-3 top-3"
              />
            </Dialog.Close>
          </Dialog.Content>
        </div>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

// Хук-машина фазы (владелец — секция contact): открытие отменяет идущее
// закрытие (повторный клик по кнопке браузера в 250мс-окно не оставляет
// модалку в exit-классах), закрытие идемпотентно. Структура — канон
// openModal/close из feedback-modal.
export function useInstallModal() {
  const [browser, setBrowser] = useState<BrowserId | null>(null);
  const [closing, setClosing] = useState(false);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (closeTimer.current) {
        clearTimeout(closeTimer.current);
      }
    },
    [],
  );

  const open = (next: BrowserId) => {
    if (closeTimer.current) {
      clearTimeout(closeTimer.current);
      closeTimer.current = null;
    }
    setClosing(false);
    setBrowser(next);
  };

  // Идемпотентность: Radix бьёт onOpenChange(false) дважды на один клик
  // по фону (pointer-down-outside + focus-outside — фаза closing держит
  // слой смонтированным), повторный вход не перезаводит таймер.
  const close = () => {
    if (closeTimer.current || browser === null) {
      return;
    }
    setClosing(true);
    closeTimer.current = setTimeout(() => {
      closeTimer.current = null;
      // closing не сбрасываем: сброс в этом же коммите возвращает классам
      // карточки входную анимацию на ещё живом оверлее (канон feedback-modal).
      setBrowser(null);
    }, CLOSE_MS);
  };

  return { browser, closing, open, close };
}

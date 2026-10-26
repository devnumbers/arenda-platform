"use client";

import { useEffect, useState } from "react";
import { Toaster, toast } from "sonner";
import { cn } from "@/lib/cn";
import { LandingButton } from "./button";

// Копирование почты поддержки в буфер с тостом «Почта скопирована»
// (решение владельца 26.10). Пилюля тоста — компонент 3647-90729
// (раскладки 3647-88916 ПК / 3647-162433 мобайл): радиус 12, белый фон,
// тень shadow-card (совпадает с переменной кадра: 0 8px 24px rgb(0 0 0/0.12)),
// текст 12/15 ink, паддинги 16 слева / 12 справа / 8 по вертикали,
// иконка 24×24 с зазором 4. На ПК/планшете тост прилетает вверху справа
// (top 104 — 32 под нижним краем хедер-бара 72px; right 32), на мобайле —
// вверху по центру.
const SUPPORT_EMAIL = "hello@rentlee.ru";
const TOAST_DURATION_MS = 3000;
// Отступы тоста от краёв вьюпорта — по кадрам раскладок; дублируются в
// mobileOffset, потому что медиа sonner ≤600px переопределяет var-ы.
const TOAST_OFFSET = { top: 104, right: 32, left: 32 };

async function copyToClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    // Не-secure контекст или отказ в разрешении — запасной путь
    // через скрытую textarea (prod и стенд https, нужен редко).
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.setAttribute("readonly", "");
    textarea.style.position = "fixed";
    textarea.style.opacity = "0";
    document.body.appendChild(textarea);
    textarea.select();
    let copied = false;
    try {
      copied = document.execCommand("copy");
    } catch {
      copied = false;
    }
    textarea.remove();
    return copied;
  }
}

async function copySupportEmail(): Promise<void> {
  if (await copyToClipboard(SUPPORT_EMAIL)) showCopiedToast();
}

// Без стабильного id каждый клик рождает свой тост — стопка до 3
// видимых (visibleToasts sonner по умолчанию), у каждого свой таймер,
// лишние ждут в очереди (решение владельца 26.10 — наслаивание).
function showCopiedToast(): void {
  toast.custom(() => <CopiedToast />, { duration: TOAST_DURATION_MS });
}

// Обёртка держит выравнивание: у sonner на ≤600px тост-контейнер
// растягивается на ширину минус отступы, поэтому до 480 (мобильный
// ярус лендинга) пилюля по центру, на 481–600 — справа, как на ПК.
// Граница 480 живёт и в matchMedia хоста ниже — менять согласованно.
function CopiedToast() {
  return (
    <div className="flex justify-end max-tab:justify-center">
      <div className="flex h-10 items-center gap-1 rounded-xl bg-white py-2 pr-3 pl-4 shadow-card">
        <span className="text-[12px] leading-[15px] text-ink">
          Почта скопирована
        </span>
        <GoodCheck className="size-6 shrink-0" />
      </div>
    </div>
  );
}

// Хост тостов sonner. У sonner своя мобильная граница (600px), а ярусы
// лендинга — 480/481: позицию переключаем сами по matchMedia, отступы
// дублируются в mobileOffset (медиа sonner ≤600px переопределяет var-ы
// отступов). unstyled снимает дефолтную карточку sonner — визуал только
// наш. По умолчанию SSR рендерит десктопную позицию; тост появляется
// только по клику, поэтому вспышки на мобиле нет.
export function EmailToastHost() {
  const [mobile, setMobile] = useState(false);

  useEffect(() => {
    const mq = window.matchMedia("(max-width: 480px)");
    const update = () => setMobile(mq.matches);
    update();
    mq.addEventListener("change", update);
    return () => mq.removeEventListener("change", update);
  }, []);

  return (
    <Toaster
      position={mobile ? "top-center" : "top-right"}
      offset={TOAST_OFFSET}
      mobileOffset={TOAST_OFFSET}
      duration={TOAST_DURATION_MS}
      toastOptions={{ unstyled: true }}
    />
  );
}

// Синяя почта в бегущем тексте (ответ FAQ, тела юрстраниц) — как ссылка
// политик: #2b7fff, hover #2175f5, active #176beb, без подчёркивания
// (прецедент — ссылка cookie-баннера, включая её нативную фокус-обводку:
// outline-none здесь съел бы единственный индикатор фокуса — цвет уже
// синий), кнопка вместо mailto.
export function CopyEmailLink({ className }: { className?: string }) {
  return (
    <button
      type="button"
      onClick={() => {
        void copySupportEmail();
      }}
      className={cn(
        "cursor-pointer text-primary transition-colors duration-200 active:duration-75 hover:text-primary-hover active:text-primary-active",
        className,
      )}
    >
      {SUPPORT_EMAIL}
    </button>
  );
}

// Бегущий текст с почтой поддержки: почта вырезается в синюю кнопку
// копирования, остальной текст остаётся как есть. Строки контента не
// меняются — их же читает json-ld (FAQPage).
export function CopyEmailText({ text }: { text: string }) {
  const emailAt = text.indexOf(SUPPORT_EMAIL);
  if (emailAt === -1) {
    return <>{text}</>;
  }
  return (
    <>
      {text.slice(0, emailAt)}
      <CopyEmailLink />
      {text.slice(emailAt + SUPPORT_EMAIL.length)}
    </>
  );
}

// Кнопка почты в футере — тот же gray-инстанс LandingButton, что был
// у mailto-ссылки, действие — копирование вместо почтового клиента.
export function CopyEmailButton() {
  return (
    <LandingButton
      variant="gray"
      onClick={() => {
        void copySupportEmail();
      }}
    >
      {SUPPORT_EMAIL}
    </LandingButton>
  );
}

// Иконка «печать с галочкой» — оригинальный SVG из Figma
// (Icon/Color/GoodWhite, кадр 3647-162423): градиент #3ED163→#20B345,
// белая обводка и галочка. Id градиента и фильтра с префиксом —
// инлайн-SVG не должен пересекаться по id с другими инстансами.
function GoodCheck({ className }: { className?: string }) {
  return (
    <svg
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
      className={className}
    >
      <g filter="url(#good-check-inner-shadow)">
        <path
          d="M10.1905 5.2795C9.67865 5.79136 9.17142 6.02901 8.44166 6.02901C7.80448 6.02901 6.89794 5.90543 6.4 6.40756C5.906 6.90571 6.02902 7.80852 6.02902 8.44164C6.02902 9.17142 5.79136 9.67865 5.27948 10.1905C4.4265 11.0435 4.00001 11.47 4 12C4.00001 12.53 4.42651 12.9565 5.2795 13.8095C5.85292 14.3829 6.02902 14.7531 6.02902 15.5583C6.02902 16.1955 5.90545 17.1021 6.40759 17.6C6.90575 18.094 7.80854 17.971 8.44165 17.971C9.21879 17.971 9.59303 18.123 10.1477 18.6776C10.6199 19.1499 11.253 20 12 20C12.747 20 13.3801 19.1499 13.8523 18.6776C14.407 18.123 14.7812 17.971 15.5583 17.971C16.1915 17.971 17.0943 18.094 17.5924 17.6C18.0946 17.1021 17.971 16.1955 17.971 15.5583C17.971 14.7531 18.1471 14.3829 18.7205 13.8095C19.5735 12.9565 20 12.53 20 12C20 11.47 19.5735 11.0435 18.7205 10.1905C18.1471 9.6171 17.971 9.24686 17.971 8.44164C17.971 7.80447 18.0946 6.89795 17.5924 6.4C17.0943 5.90598 16.1915 6.02901 15.5583 6.02901C14.8284 6.02901 14.3212 5.79125 13.8095 5.27949C12.9565 4.4265 12.53 4 12 4C11.47 4 11.0435 4.4265 10.1905 5.2795Z"
          fill="url(#good-check-gradient)"
        />
      </g>
      <path
        d="M12.2363 2.7627C12.7722 2.81999 13.2001 3.06535 13.542 3.32617C13.9038 3.60223 14.2919 3.99407 14.6934 4.39551C14.9966 4.69872 15.195 4.7793 15.5586 4.7793C15.7265 4.77926 16.3033 4.74705 16.7314 4.7832C17.1736 4.82056 17.8248 4.94025 18.3662 5.41309L18.4727 5.5127L18.5732 5.61914C19.0538 6.16065 19.1772 6.81489 19.2158 7.26074C19.2532 7.69263 19.2207 8.27089 19.2207 8.44141C19.2207 8.75617 19.2569 8.85777 19.2764 8.90234C19.2997 8.95579 19.359 9.06111 19.6045 9.30664C20.0059 9.70808 20.3978 10.0962 20.6738 10.458C20.9719 10.8487 21.25 11.3518 21.25 12L21.2373 12.2363C21.18 12.7722 20.9346 13.2001 20.6738 13.542C20.3978 13.9038 20.0059 14.2919 19.6045 14.6934C19.359 14.9388 19.2997 15.0442 19.2764 15.0977C19.2569 15.1423 19.2207 15.2439 19.2207 15.5586C19.2207 15.7291 19.2532 16.3074 19.2158 16.7393C19.1747 17.2148 19.0372 17.9273 18.4727 18.4873C17.9123 19.043 17.203 19.176 16.7314 19.2158C16.3033 19.252 15.7264 19.2207 15.5586 19.2207C15.2442 19.2207 15.1423 19.2535 15.0986 19.2715C15.0553 19.2894 14.9594 19.3385 14.7363 19.5615C14.6282 19.6697 14.1869 20.164 13.8516 20.457C13.5049 20.7601 12.9214 21.1925 12.1553 21.2451L12 21.25C11.1573 21.25 10.5182 20.7802 10.1484 20.457C9.81313 20.164 9.37183 19.6697 9.26367 19.5615C9.04064 19.3385 8.94474 19.2894 8.90137 19.2715C8.85774 19.2535 8.75581 19.2207 8.44141 19.2207C8.27356 19.2207 7.6967 19.252 7.26855 19.2158C6.79699 19.176 6.08773 19.043 5.52734 18.4873C4.96277 17.9273 4.82532 17.2148 4.78418 16.7393C4.74683 16.3074 4.77926 15.7292 4.7793 15.5586C4.7793 15.2439 4.74308 15.1422 4.72363 15.0977C4.70029 15.0442 4.64101 14.9389 4.39551 14.6934C3.99406 14.2919 3.60223 13.9038 3.32617 13.542C3.06535 13.2001 2.82 12.7722 2.7627 12.2363L2.75 12L2.7627 11.7637C2.81999 11.2278 3.06535 10.7999 3.32617 10.458C3.60223 10.0962 3.99406 9.70808 4.39551 9.30664C4.69878 9.00336 4.7793 8.80486 4.7793 8.44141C4.77927 8.27356 4.74706 7.69672 4.7832 7.26855C4.82304 6.79698 4.95694 6.08777 5.5127 5.52734L5.61914 5.42676C6.16065 4.94615 6.81488 4.82275 7.26074 4.78418C7.69262 4.74683 8.27087 4.77927 8.44141 4.7793C8.80486 4.7793 9.0034 4.69875 9.30664 4.39551C9.70808 3.99407 10.0962 3.60223 10.458 3.32617C10.8487 3.02809 11.3518 2.75 12 2.75L12.2363 2.7627Z"
        stroke="white"
        strokeWidth="2.5"
      />
      <path
        d="M14.3353 9.22017C14.545 8.96369 14.9233 8.92546 15.1798 9.13501C15.4364 9.34458 15.4744 9.72291 15.265 9.97954L11.3447 14.7795C11.2403 14.9073 11.0874 14.9864 10.9228 14.9983C10.7582 15.01 10.596 14.9536 10.4744 14.842L8.79469 13.2991C8.55072 13.075 8.5341 12.6955 8.75797 12.4514C8.98201 12.2076 9.3616 12.191 9.60563 12.4147L10.8173 13.5272L14.3353 9.22017Z"
        fill="white"
      />
      <defs>
        <filter
          id="good-check-inner-shadow"
          x="1.5"
          y="1.5"
          width="21"
          height="21"
          filterUnits="userSpaceOnUse"
          colorInterpolationFilters="sRGB"
        >
          <feFlood floodOpacity="0" result="BackgroundImageFix" />
          <feBlend
            mode="normal"
            in="SourceGraphic"
            in2="BackgroundImageFix"
            result="shape"
          />
          <feColorMatrix
            in="SourceAlpha"
            type="matrix"
            values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0"
            result="hardAlpha"
          />
          <feOffset />
          <feGaussianBlur stdDeviation="1.5" />
          <feComposite in2="hardAlpha" operator="arithmetic" k2="-1" k3="1" />
          <feColorMatrix
            type="matrix"
            values="0 0 0 0 1 0 0 0 0 1 0 0 0 0 1 0 0 0 1 0"
          />
          <feBlend
            mode="normal"
            in2="shape"
            result="effect1_innerShadow_3745_80764"
          />
        </filter>
        <linearGradient
          id="good-check-gradient"
          x1="12"
          y1="4"
          x2="12"
          y2="20"
          gradientUnits="userSpaceOnUse"
        >
          <stop stopColor="#3ED163" />
          <stop offset="1" stopColor="#20B345" />
        </linearGradient>
      </defs>
    </svg>
  );
}

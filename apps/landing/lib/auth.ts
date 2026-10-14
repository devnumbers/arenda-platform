import { cache } from "react";
import { headers } from "next/headers";

// Состояние авторизации для tariff-aware хедера (ADR 0063): сервер читает
// сессионную cookie кабинета и ходит на GET /me напрямую через BACKEND_URL
// (compose-сеть в stage/prod, слотовый бэк локально). Любая ошибка/таймаут —
// fail-open в гостевой хедер; тариф нужен только именем. cache(): layout и
// страница рендерятся в одном запросе — /me зовётся один раз.
export type TariffName = "basic" | "pro" | "business";

// Период подписки — словарь кабинета (subscription.currentPeriod),
// не словарь лендинга (yearly/monthly).
export type SubscriptionPeriod = "month" | "year";

export type Me = { tariff?: TariffName; period?: SubscriptionPeriod };

export const getMe = cache(async (): Promise<Me | null> => {
  const cookieHeader = (await headers()).get("cookie") ?? "";
  // Имена cookie кабинета: __Host-session_id (secure) / session_id.
  if (!cookieHeader.includes("session_id")) {
    return null;
  }
  try {
    const res = await fetch(
      `${process.env.BACKEND_URL ?? "http://localhost:8080"}/me`,
      {
        headers: { cookie: cookieHeader },
        signal: AbortSignal.timeout(1500),
        cache: "no-store",
      },
    );
    if (!res.ok) {
      return null;
    }
    const data = (await res.json()) as {
      subscription?: {
        tariff?: { name?: TariffName };
        currentPeriod?: string;
      };
    };
    const period = data.subscription?.currentPeriod;
    return {
      tariff: data.subscription?.tariff?.name,
      period:
        period === "month" || period === "year" ? period : undefined,
    };
  } catch {
    return null;
  }
});

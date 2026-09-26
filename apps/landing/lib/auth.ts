import { headers } from "next/headers";

// Состояние авторизации для tariff-aware хедера (ADR 0063): сервер читает
// сессионную cookie кабинета и ходит на GET /me напрямую через BACKEND_URL
// (compose-сеть в stage/prod, слотовый бэк локально). Любая ошибка/таймаут —
// fail-open в гостевой хедер; тариф нужен только именем.
export type TariffName = "basic" | "pro" | "business";

export type Me = { tariff?: TariffName };

export const TARIFF_TITLES: Record<TariffName, string> = {
  basic: "Базовый",
  pro: "Про",
  business: "Бизнес",
};

export async function getMe(): Promise<Me | null> {
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
      subscription?: { tariff?: TariffName };
    };
    return { tariff: data.subscription?.tariff };
  } catch {
    return null;
  }
}

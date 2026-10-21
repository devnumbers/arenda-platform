// Согласие на файлы cookie лендинга — одна кука: её ставит клик «Хорошо»
// баннера (components/cookie-banner.tsx), её читает SSR (site)/layout.tsx,
// чтобы согласовавшему баннер не рендерился вовсе, а не прятался гидрацией.
export const COOKIE_CONSENT_COOKIE = "cookie_consent";
export const COOKIE_CONSENT_ACCEPTED = "accepted";

// 12 месяцев — общепринятый срок хранения согласия (практика EDPB:
// согласие переспрашивают, а не хранят вечно); по истечении куки баннер
// возвращается.
export const COOKIE_CONSENT_MAX_AGE = 60 * 60 * 24 * 365;

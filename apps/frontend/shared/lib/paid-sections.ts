import { ROUTES } from '@/shared/config/routes';
import type { TariffName } from '@/shared/model/tariff';

/** Платные разделы кабинета (карта #997, ADR 0064): маршруты, доступные
 * только платным тарифам. proxy.ts вызывает гейт на каждом /me-гейтованном
 * маршруте: null-подписка считается базовым тарифом, grace/cancelled на
 * платном тарифе имя сохраняют — статус не проверяется. Обход мимо UI ловит
 * бекенд-гейт (402 tariff_required); реестры фронта и бека меняются парно —
 * каждый новый платный экран добавляет паттерн здесь и переопределение
 * роутов в httpserver/server.go. Цель редиректа — страница выбора тарифа
 * без контекста (решение владельца 30.09: плашка-контекст в дизайне не
 * нужна, снесена вместе с query gate). */
const PAID_SECTION_ROUTES: ReadonlyArray<{ readonly pattern: RegExp }> = [
    {
        // Экран 1 карты #997: «Совместный доступ объекта» — список участников
        // и приглашение.
        pattern: /^\/properties\/[^/]+\/participants(?:\/|$)/,
    },
    {
        // Экран 2 карты #997: «Участники» — глобальный хаб и все вложенные,
        // включая глобальную форму приглашения (POST /participants/invite).
        pattern: /^\/participants(?:\/|$)/,
    },
    {
        // Экран 3 карты #997: «История действий» — лента и вложенные страницы
        // участника и объекта; фильтры — шит на хабе, их опции обслуживает
        // гейтуемый бекенд-эндпойнт GET /history/filters.
        pattern: /^\/history(?:\/|$)/,
    },
];

/** Путь редиректа на выбор тарифа или null, когда маршрут не платный либо
 * тариф платный. */
export function paidSectionGateRedirect(
    pathname: string,
    tariffName: TariffName | null,
): string | null {
    const gated = PAID_SECTION_ROUTES.some(({ pattern }) =>
        pattern.test(pathname),
    );
    if (!gated) {
        return null;
    }
    if (tariffName !== null && tariffName !== 'basic') {
        return null;
    }
    return ROUTES.profileTariffChange;
}

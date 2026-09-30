import { ROUTES } from '@/shared/config/routes';
import type { TariffName } from '@/shared/model/tariff';

/** Платные разделы кабинета (карта #997, ADR 0064): маршруты, доступные
 * только платным тарифам. proxy.ts вызывает гейт на каждом /me-гейтованном
 * маршруте: null-подписка считается базовым тарифом, grace/cancelled на
 * платном тарифе имя сохраняют — статус не проверяется. Обход мимо UI ловит
 * бекенд-гейт (402 tariff_required); реестры фронта и бека меняются парно —
 * каждый новый платный экран добавляет паттерн здесь и переопределение
 * роутов в httpserver/server.go. */
const PAID_SECTION_ROUTES: ReadonlyArray<{
    readonly pattern: RegExp;
    readonly gate: string;
}> = [
    {
        // Экран 1 карты #997: «Совместный доступ объекта» — список участников
        // и приглашение.
        pattern: /^\/properties\/[^/]+\/participants(?:\/|$)/,
        gate: 'property-participants',
    },
    {
        // Экран 2 карты #997: «Участники» — глобальный хаб и все вложенные,
        // включая глобальную форму приглашения (POST /participants/invite).
        pattern: /^\/participants(?:\/|$)/,
        gate: 'participants',
    },
    {
        // Экран 3 карты #997: «История действий» — лента и вложенные страницы
        // участника и объекта; фильтры — шит на хабе, их опции обслуживает
        // гейтуемый бекенд-эндпойнт GET /history/filters.
        pattern: /^\/history(?:\/|$)/,
        gate: 'history',
    },
];

/** Путь редиректа на выбор тарифа с query-контекстом гейта или null, когда
 * маршрут не платный либо тариф платный. */
export function paidSectionGateRedirect(
    pathname: string,
    tariffName: TariffName | null,
): string | null {
    const section = PAID_SECTION_ROUTES.find(({ pattern }) =>
        pattern.test(pathname),
    );
    if (section === undefined) {
        return null;
    }
    if (tariffName !== null && tariffName !== 'basic') {
        return null;
    }
    return `${ROUTES.profileTariffChange}?gate=${section.gate}`;
}

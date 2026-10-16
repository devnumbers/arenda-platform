/**
 * Название тарифа подписки. Живёт в shared, потому что на него ссылаются
 * несколько entity-слайсов (user, billing), а cross-slice импорты в слое
 * entities запрещены (enforced by boundaries/dependencies в eslint.config.mjs).
 */
export type TariffName = 'basic' | 'pro' | 'business';

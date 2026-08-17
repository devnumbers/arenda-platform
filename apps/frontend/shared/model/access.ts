/**
 * Роль доступа к объекту аренды. Живёт в shared, потому что на неё ссылаются
 * несколько entity-слайсов (property, access), а cross-slice импорты в слое
 * entities запрещены (enforced by boundaries/dependencies в eslint.config.mjs).
 */
export type AccessRole = 'owner' | 'full_access' | 'viewer';

/** Роли участника совместного доступа (владелец не является membership). */
export type SharedAccessRole = Exclude<AccessRole, 'owner'>;

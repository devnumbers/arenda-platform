/**
 * Роль доступа к объекту аренды. Живёт в shared, потому что на неё ссылаются
 * несколько entity-слайсов (property, access), а cross-slice импорты в слое
 * entities запрещены (enforced by boundaries/dependencies в eslint.config.mjs).
 */
export type AccessRole = 'owner' | 'full_access' | 'viewer';

/** Роли участника совместного доступа (владелец не является membership). */
export type SharedAccessRole = Exclude<AccessRole, 'owner'>;

/** Словарь отображения ролей — решение чарта карты #692: роли только
 * отображение, full_access читается «Редактирование» (не «Полный
 * доступ»), viewer — «Просмотр». Единственный источник подписи роли для
 * чипов ног, пилюль доступа, пикера роли и фильтра «Участников объекта».
 * Живёт рядом с типом по той же причине, что и сам тип. */
export const ACCESS_ROLE_LABELS: Readonly<Record<SharedAccessRole, string>> = {
    viewer: 'Просмотр',
    full_access: 'Редактирование',
};

/** Имя иконки роли по канону Icon/S — Edit для «Редактирования», Eye для
 * «Просмотра»; поверхность читает имя и мапит на компонент канона иконок
 * локально (ассеты — забота слоя отображения). */
export const ACCESS_ROLE_ICON_NAMES: Readonly<Record<SharedAccessRole, 'edit' | 'eye'>> = {
    viewer: 'eye',
    full_access: 'edit',
};

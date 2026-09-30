import type { PropertyStatus } from '@/entities/property';

/**
 * Копирайт пустых состояний секций детали объекта (карта #583, тикет
 * #588). Два набора (Figma 1554:98469 — приветственные первого объекта,
 * 1554:100751 — обычные «второго объекта»); правила переключения по
 * блокам фиксируются с владельцем на приёмке — пока набор общий на
 * страницу, у аренды добавляется статусный вариант описания (1581:53679 —
 * ремонт, 1581:52407 — архив). У операций CTA нет (98469).
 */

export type PropertyDetailSectionKey =
  | 'rental'
  | 'payments'
  | 'operations'
  | 'contacts'
  | 'tasks'
  | 'about';

export type PropertyEmptySet = 'welcome' | 'regular';

export type PropertySectionEmptyCopy = {
  /** Текст состояния: в полной анатомии — тёмный тайтл 20/24, в коротком
   * (description: null) — единственная серая строка 14/16. */
  readonly title: string;
  /** Описание: онбординг-текст у первого объекта и статусный у аренды
   * (ремонт/архив); null — короткая анатомия без описания. */
  readonly description: string | null;
  readonly ctaLabel: string | null;
};

/** Иллюстрации пустых состояний секций (64): переиспользуем иллюстрации
 * пустых карт аренды/платежей/контактов/задач — совпадение с макетом 1:1;
 * характеристики — из шаблона «Об объекте» (1550:97124, нода 1550:97390). */
export const propertySectionImages: Record<PropertyDetailSectionKey, string> = {
  rental: '/images/rentals/empty-rental.webp',
  payments: '/images/payments/object-empty.webp',
  operations: '/images/payments/operations-empty.webp',
  contacts: '/images/contacts/empty-contacts.webp',
  tasks: '/images/tasks/empty-tasks.webp',
  about: '/images/properties/characteristics-empty.webp',
};

/** Набор пустых состояний страницы: единственный активный объект —
 * приветственный (Figma: «первый объект»), остальные — обычные
 * («второй объект»). */
export function resolvePropertyDetailEmptySet(activePropertyCount: number): PropertyEmptySet {
  return activePropertyCount === 1 ? 'welcome' : 'regular';
}

export function resolvePropertySectionEmpty(
  key: PropertyDetailSectionKey,
  set: PropertyEmptySet,
  status: PropertyStatus,
): PropertySectionEmptyCopy {
  // Аренда — единственный блок со статусными описаниями: у ремонта и
  // архива полная анатомия с описанием независимо от набора (решение
  // владельца 11.09, Figma 1581:53679 / 1581:52407).
  if (key === 'rental') {
    const description =
      status === 'active'
        ? set === 'welcome'
          ? 'Укажите арендную ставку и сроки действия договора'
          : null
        : 'Добавьте условия аренды и настройте платеж';
    return { title: 'Аренда не добавлена', description, ctaLabel: 'Добавить' };
  }

  if (key === 'payments') {
    return {
      title: 'Платежи не добавлены',
      description:
        set === 'welcome'
          ? 'Добавьте регулярные платежи: коммунальные услуги, кредит или взносы'
          : null,
      ctaLabel: 'Добавить',
    };
  }

  if (key === 'operations') {
    return {
      title: 'Операций еще не было',
      description:
        set === 'welcome'
          ? 'Здесь появятся записи об оплате, ремонте и других операциях'
          : null,
      ctaLabel: null,
    };
  }

  if (key === 'contacts') {
    return {
      title: 'Контакты не добавлены',
      description:
        set === 'welcome'
          ? 'Добавьте контакты арендаторов, мастеров и других специалистов'
          : null,
      ctaLabel: 'Добавить',
    };
  }

  if (key === 'tasks') {
    return {
      title: 'Задач нет',
      description:
        set === 'welcome'
          ? 'Добавьте задачу — напоминание, звонок или вызов мастера'
          : null,
      ctaLabel: 'Добавить',
    };
  }

  // about (секция «Квартира»): у первого объекта — с онбординг-описанием
  // (98469), у второго и далее — короткая (1550:97124).
  return {
    title: 'Характеристики не добавлены',
    description:
      set === 'welcome' ? 'Укажите площадь, этаж и другие параметры объекта' : null,
    ctaLabel: 'Добавить',
  };
}

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
  readonly title: string;
  readonly description: string | null;
  readonly ctaLabel: string | null;
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
  if (key === 'rental') {
    const description =
      status === 'active'
        ? 'Укажите арендную ставку и сроки действия договора'
        : 'Добавьте условия аренды и настройте платеж';
    return { title: 'Аренда не добавлена', description, ctaLabel: 'Добавить' };
  }

  if (key === 'payments') {
    return {
      title: 'Платежи не добавлены',
      description: 'Добавьте регулярные платежи: коммунальные услуги, кредит или взносы',
      ctaLabel: 'Добавить',
    };
  }

  if (key === 'operations') {
    return {
      title: 'Операций еще не было',
      description: 'Здесь появятся записи об оплате, ремонте и других операциях',
      ctaLabel: null,
    };
  }

  if (key === 'contacts') {
    return {
      title: 'Контакты не добавлены',
      description: 'Добавьте контакты арендаторов, мастеров и других специалистов',
      ctaLabel: 'Добавить',
    };
  }

  if (key === 'tasks') {
    return {
      title: 'Задач нет',
      description: 'Добавьте задачу — напоминание, звонок или вызов мастера',
      ctaLabel: 'Добавить',
    };
  }

  // about (секция «Квартира»): приветственное — с онбординг-описанием
  // (98469), обычное — короткое, без описания (канон 1550:97124).
  if (set === 'welcome') {
    return {
      title: 'Характеристики не добавлены',
      description: 'Укажите площадь, этаж и другие параметры объекта',
      ctaLabel: 'Добавить',
    };
  }
  return { title: 'Характеристики не добавлены', description: null, ctaLabel: 'Добавить' };
}

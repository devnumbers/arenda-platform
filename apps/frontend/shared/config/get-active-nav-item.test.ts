import { describe, expect, it } from 'vitest';
import { HomeMain, Wallet } from '@/shared/assets/icons';
import { getActiveNavItem, getActiveMobileTab } from './get-active-nav-item';
import { allNavSections, type NavSection } from './navigation';

const activeSectionId = (pathname: string): string | null =>
  getActiveNavItem(pathname)?.id ?? null;

describe('правила активности навигации (#558)', () => {
  it('«Объекты»: корень, служебные страницы, страница объекта и все её внутренние страницы', () => {
    expect(activeSectionId('/properties')).toBe('properties');
    expect(activeSectionId('/properties/new')).toBe('properties');
    expect(activeSectionId('/properties/archive')).toBe('properties');
    expect(activeSectionId('/properties/42')).toBe('properties');
    expect(activeSectionId('/properties/42/payments')).toBe('properties');
    expect(activeSectionId('/properties/42/payments/7/edit')).toBe('properties');
    expect(activeSectionId('/properties/42/operations/9')).toBe('properties');
    expect(activeSectionId('/properties/42/contacts')).toBe('properties');
  });

  it('глобальные разделы — свой пункт на любой глубине', () => {
    expect(activeSectionId('/operations')).toBe('operations');
    expect(activeSectionId('/operations/search')).toBe('operations');
    expect(activeSectionId('/operations/expenses')).toBe('operations');
    expect(activeSectionId('/operations/objects')).toBe('operations');
    expect(activeSectionId('/tasks')).toBe('tasks');
    expect(activeSectionId('/tasks/new')).toBe('tasks');
    expect(activeSectionId('/tasks/abc/edit')).toBe('tasks');
    expect(activeSectionId('/contacts')).toBe('contacts');
    expect(activeSectionId('/contacts/search')).toBe('contacts');
  });

  it('«Уведомления» — только раздел /profile/notifications, не весь профиль', () => {
    expect(activeSectionId('/profile/notifications')).toBe('notifications');
    expect(activeSectionId('/profile/notifications/settings')).toBe('notifications');
    expect(activeSectionId('/profile')).toBeNull();
    expect(activeSectionId('/profile/account')).toBeNull();
    expect(activeSectionId('/profile/tariff')).toBeNull();
  });

  it('«Платежи» и «Участники» — по своему префиксу', () => {
    expect(activeSectionId('/payments')).toBe('payments');
    expect(activeSectionId('/participants')).toBe('participants');
  });

  it('остальное — без активного пункта («Еще» на мобайле, без подсветки на десктопе)', () => {
    expect(activeSectionId('/')).toBeNull();
    expect(activeSectionId('/login')).toBeNull();
    expect(activeSectionId('/subscription')).toBeNull();
    expect(activeSectionId('/ui-kit')).toBeNull();
    expect(activeSectionId('/profile/info/terms')).toBeNull();
    // «Поддержка» — действие хрома (модалка #766), а не раздел: маршрута
    // больше нет, страница снесена — активного пункта нет и быть не может.
    expect(activeSectionId('/support')).toBeNull();
    expect(activeSectionId('/support/faq')).toBeNull();
  });

  it('соседний префикс не подсвечивает раздел', () => {
    expect(activeSectionId('/operations-archive')).toBeNull();
    expect(activeSectionId('/propertiesX')).toBeNull();
    expect(activeSectionId('/paymentsHistory')).toBeNull();
  });

  it('longest-prefix: из совпадающих префиксов побеждает длиннейший', () => {
    // Гипотетическое вложение: у раздела «Платежи» свой пункт навигации
    // внутри префикса «Операций» — правило должно выбрать длиннейший.
    const sections: ReadonlyArray<NavSection> = [
      { id: 'operations', label: 'Операции', href: '/operations', Icon: HomeMain },
      { id: 'payments', label: 'Платежи', href: '/operations/payments', Icon: Wallet },
    ];

    expect(getActiveNavItem('/operations/payments/7', sections)?.id).toBe('payments');
    expect(getActiveNavItem('/operations/objects', sections)?.id).toBe('operations');
    expect(getActiveNavItem('/operations', sections)?.id).toBe('operations');
  });

  it('возвращает сам раздел из конфига — с подписью и иконкой, не только id', () => {
    const active = getActiveNavItem('/tasks');
    const expected = allNavSections.find((section) => section.id === 'tasks');

    expect(active).toBe(expected);
    expect(active?.label).toBe('Задачи');
  });
});

describe('активный таб мобильного TabBar (#560)', () => {
  it('«Объекты» — корень и внутренние страницы объекта', () => {
    expect(getActiveMobileTab('/properties')).toBe('properties');
    expect(getActiveMobileTab('/properties/new')).toBe('properties');
    expect(getActiveMobileTab('/properties/42/payments')).toBe('properties');
  });

  it('«Уведомления» — только раздел уведомлений, не весь профиль', () => {
    expect(getActiveMobileTab('/profile/notifications')).toBe('notifications');
    expect(getActiveMobileTab('/profile/notifications/settings')).toBe('notifications');
    expect(getActiveMobileTab('/profile')).toBe('more');
  });

  it('«Еще» — глобальные разделы без своего таба, профиль и прочее', () => {
    expect(getActiveMobileTab('/operations')).toBe('more');
    expect(getActiveMobileTab('/tasks/new')).toBe('more');
    expect(getActiveMobileTab('/payments')).toBe('more');
    expect(getActiveMobileTab('/contacts')).toBe('more');
    expect(getActiveMobileTab('/')).toBe('more');
    expect(getActiveMobileTab('/ui-kit')).toBe('more');
  });
});

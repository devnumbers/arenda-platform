import { describe, expect, it } from 'vitest';
import {
  CheckmarkCircle,
  HomeMain,
  NotificationSettings,
  Support,
  Team,
  TimeHistory,
  UserCircle,
  Wallet,
} from '@/shared/assets/icons';
import {
  allNavSections,
  mainNavSections,
  moreSheetNavSections,
  secondaryNavSections,
  supportNavSection,
} from './navigation';

describe('единый конфиг разделов навигации (#558)', () => {
  it('6 главных разделов в порядке сайдбара (Figma 1675:54051)', () => {
    expect(mainNavSections.map((section) => section.id)).toStrictEqual([
      'properties',
      'payments',
      'operations',
      'tasks',
      'contacts',
      'participants',
    ]);
  });

  it('подписи как в Figma', () => {
    expect(mainNavSections.map((section) => section.label)).toStrictEqual([
      'Объекты',
      'Платежи',
      'Операции',
      'Задачи',
      'Контакты',
      'Участники',
    ]);
    expect(secondaryNavSections.map((section) => section.label)).toStrictEqual([
      'Уведомления',
    ]);
  });

  it('иконки — канон-каталог (Icon/R)', () => {
    expect(mainNavSections.map((section) => section.Icon)).toStrictEqual([
      HomeMain,
      Wallet,
      TimeHistory,
      CheckmarkCircle,
      UserCircle,
      Team,
    ]);
    expect(secondaryNavSections.map((section) => section.Icon)).toStrictEqual([
      NotificationSettings,
    ]);
  });

  it('адреса разделов', () => {
    expect(allNavSections.map((section) => [section.id, section.href])).toStrictEqual([
      ['properties', '/properties'],
      ['payments', '/payments'],
      ['operations', '/operations'],
      ['tasks', '/tasks'],
      ['contacts', '/contacts'],
      ['participants', '/participants'],
      ['notifications', '/notifications'],
    ]);
  });

  it('«Поддержка» — действие хрома, не раздел с маршрутом (#766): модалка вместо страницы', () => {
    expect(supportNavSection.id).toBe('support');
    expect(supportNavSection.label).toBe('Поддержка');
    expect(supportNavSection.Icon).toBe(Support);
    expect(supportNavSection.href).toBeUndefined();
    expect(allNavSections).not.toContain(supportNavSection);
  });

  it('шит «Еще»: 5 разделов-ссылок, шестая ячейка — «Поддержка»-действие', () => {
    expect(moreSheetNavSections.map((section) => section.id)).toStrictEqual([
      'payments',
      'operations',
      'tasks',
      'contacts',
      'participants',
    ]);
    expect(moreSheetNavSections.map((section) => section.label)).toStrictEqual([
      'Платежи',
      'Операции',
      'Задачи',
      'Контакты',
      'Участники',
    ]);
    expect(moreSheetNavSections).not.toContain(supportNavSection);
  });

  it('шит «Еще»: без табов TabBar («Объекты», «Уведомления»); адреса уникальны', () => {
    const ids = moreSheetNavSections.map((section) => section.id);
    const hrefs = moreSheetNavSections.map((section) => section.href);

    expect(ids).not.toContain('properties');
    expect(ids).not.toContain('notifications');
    expect(new Set(hrefs).size).toBe(hrefs.length);
  });

  it('id и адреса уникальны; общий список — главные, затем сервисные', () => {
    const ids = allNavSections.map((section) => section.id);
    const hrefs = allNavSections.map((section) => section.href);
    expect(new Set(ids).size).toBe(ids.length);
    expect(new Set(hrefs).size).toBe(hrefs.length);
    expect(allNavSections).toStrictEqual([...mainNavSections, ...secondaryNavSections]);
  });
});

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
import { allNavSections, mainNavSections, secondaryNavSections } from './navigation';

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
      'Поддержка',
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
      Support,
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
      ['notifications', '/profile/notifications'],
      ['support', '/support'],
    ]);
  });

  it('id и адреса уникальны; общий список — главные, затем сервисные', () => {
    const ids = allNavSections.map((section) => section.id);
    const hrefs = allNavSections.map((section) => section.href);
    expect(new Set(ids).size).toBe(ids.length);
    expect(new Set(hrefs).size).toBe(hrefs.length);
    expect(allNavSections).toStrictEqual([...mainNavSections, ...secondaryNavSections]);
  });
});

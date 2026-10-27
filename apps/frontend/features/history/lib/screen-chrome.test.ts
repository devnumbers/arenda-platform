import { describe, expect, it } from 'vitest';

import { ROUTES } from '@/shared/config/routes';

import { historyScreenBackHref, historyScreenTitle } from './screen-chrome';

describe('historyScreenTitle', () => {
  it('пара человек+объект — «Действия участника в объекте» (#841)', () => {
    expect(historyScreenTitle('user-uuid', 'property-uuid')).toBe('Действия участника в объекте');
  });

  it('объект — «История объекта» (#840)', () => {
    expect(historyScreenTitle(undefined, 'property-uuid')).toBe('История объекта');
  });

  it('страница участника — «Действия участника» (карта #1582)', () => {
    expect(historyScreenTitle('user-uuid', undefined)).toBe('Действия участника');
  });

  it('общая лента — «История действий»', () => {
    expect(historyScreenTitle(undefined, undefined)).toBe('История действий');
  });
});

describe('historyScreenBackHref', () => {
  it('пара — «Права участника» (#841)', () => {
    expect(historyScreenBackHref('user-uuid', 'property-uuid')).toBe(
      ROUTES.participantRights('user-uuid', 'property-uuid'),
    );
  });

  it('объект — «Участники объекта» (#840)', () => {
    expect(historyScreenBackHref(undefined, 'property-uuid')).toBe(
      ROUTES.propertyParticipants('property-uuid'),
    );
  });

  it('страница участника — на страницу участника (карта #1582)', () => {
    expect(historyScreenBackHref('user-uuid', undefined)).toBe(ROUTES.participant('user-uuid'));
  });

  it('общая лента — хаб «Совместный доступ»', () => {
    expect(historyScreenBackHref(undefined, undefined)).toBe(ROUTES.participants);
  });
});

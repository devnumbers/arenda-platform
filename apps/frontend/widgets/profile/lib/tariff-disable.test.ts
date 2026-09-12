import { describe, expect, it } from 'vitest';
import {
  disableConfirmTitle,
  disableSuccessTitle,
  tariffDisableObjectsCard,
} from './tariff-disable';

describe('tariffDisableObjectsCard', () => {
  it('more than one active property: mock copy with the picker sentence', () => {
    expect(tariffDisableObjectsCard(5)).toStrictEqual({
      title: 'У вас 5 объектов',
      description:
        'После отключения все объекты, кроме одного, перейдут в архив — данные сохранятся, но пользоваться ими будет нельзя. Приглашенные участники потеряют доступ к вашим объектам. Выберите объект, который останется активным',
      showPicker: true,
    });
  });

  it('two properties still offer the choice', () => {
    expect(tariffDisableObjectsCard(2).showPicker).toBe(true);
  });

  it('single property: no picker, copy without the archive and selection clauses', () => {
    expect(tariffDisableObjectsCard(1)).toStrictEqual({
      title: 'У вас 1 объект',
      description:
        'После отключения данные сохранятся. Приглашенные участники потеряют доступ к вашим объектам.',
      showPicker: false,
    });
  });

  it('zero properties: pluralized title, no picker', () => {
    const card = tariffDisableObjectsCard(0);
    expect(card.title).toBe('У вас 0 объектов');
    expect(card.showPicker).toBe(false);
  });

  it('pluralizes 2-4 properties', () => {
    expect(tariffDisableObjectsCard(2).title).toBe('У вас 2 объекта');
  });
});

describe('disableConfirmTitle', () => {
  it('names the tariff in the question', () => {
    expect(disableConfirmTitle('pro')).toBe('Уверены, что хотите отключить тариф Про?');
    expect(disableConfirmTitle('business')).toBe(
      'Уверены, что хотите отключить тариф Бизнес?',
    );
  });
});

describe('disableSuccessTitle', () => {
  it('names the disabled tariff', () => {
    expect(disableSuccessTitle('pro')).toBe('Тариф Про отключен');
    expect(disableSuccessTitle('business')).toBe('Тариф Бизнес отключен');
  });
});

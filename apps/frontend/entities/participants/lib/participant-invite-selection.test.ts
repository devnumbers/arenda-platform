import { describe, expect, it } from 'vitest';
import {
  allInvitedProperties,
  collapsedInviteRows,
  inviteSelectionState,
  toggleAllInvitedProperties,
  toggleInvitedProperty,
} from './participant-invite-selection';

const APARTMENT = '33333333-3333-4333-8333-333333333333';
const GARAGE = '44444444-4444-4444-8444-444444444444';
const STUDIO = '46464646-4646-4646-8646-464646464646';

const OPTION_IDS = [APARTMENT, GARAGE, STUDIO];

describe('allInvitedProperties — дефолт приглашения «Все объекты»', () => {
  it('выбирает все текущие объекты — снапшот на момент приглашения', () => {
    const selected = allInvitedProperties(OPTION_IDS);
    expect([...selected].sort()).toEqual([...OPTION_IDS].sort());
  });

  it('без объектов — пустой выбор', () => {
    expect(allInvitedProperties([]).size).toBe(0);
  });
});

describe('inviteSelectionState — tri-state чекбокса «Все объекты» (макет 2008-46627)', () => {
  it('пустой выбор — none', () => {
    expect(inviteSelectionState(new Set(), OPTION_IDS)).toBe('none');
  });

  it('все объекты — all', () => {
    expect(inviteSelectionState(new Set(OPTION_IDS), OPTION_IDS)).toBe('all');
  });

  it('часть объектов — partial (синий минус в макете)', () => {
    expect(inviteSelectionState(new Set([GARAGE]), OPTION_IDS)).toBe('partial');
  });

  it('ноль объектов и пустой выбор — none, а не all', () => {
    expect(inviteSelectionState(new Set(), [])).toBe('none');
  });
});

describe('toggleAllInvitedProperties — тап по строке «Все объекты»', () => {
  it('при полном выборе снимает всё', () => {
    const next = toggleAllInvitedProperties(new Set(OPTION_IDS), OPTION_IDS);
    expect(next.size).toBe(0);
  });

  it('при частичном и пустом выборе чекает все', () => {
    expect([...toggleAllInvitedProperties(new Set([GARAGE]), OPTION_IDS)].sort())
      .toEqual([...OPTION_IDS].sort());
    expect(toggleAllInvitedProperties(new Set(), OPTION_IDS).size).toBe(3);
  });
});

describe('toggleInvitedProperty — тап по строке объекта', () => {
  it('добавляет и снимает объект, не трогая остальные', () => {
    const added = toggleInvitedProperty(new Set([APARTMENT]), GARAGE);
    expect([...added].sort()).toEqual([APARTMENT, GARAGE].sort());

    const removed = toggleInvitedProperty(added, APARTMENT);
    expect([...removed]).toEqual([GARAGE]);
  });
});

describe('collapsedInviteRows — свёрнутая сводка выбора (макеты 2008-46375 / 2008-80773)', () => {
  const options = OPTION_IDS.map((id) => ({ id, name: `Объект ${id}` }));

  it('все выбранные — одна строка «Все объекты» («Все N объектов»)', () => {
    expect(collapsedInviteRows(options, new Set(OPTION_IDS))).toEqual([{ kind: 'all' }]);
  });

  it('частичный выбор — по строке на выбранный объект, в порядке списка', () => {
    const rows = collapsedInviteRows(options, new Set([STUDIO, APARTMENT]));
    expect(rows).toEqual([
      { kind: 'property', option: options[0] },
      { kind: 'property', option: options[2] },
    ]);
  });

  it('один выбранный — одна строка объекта (макет 2008-46375)', () => {
    expect(collapsedInviteRows(options, new Set([GARAGE]))).toEqual([
      { kind: 'property', option: options[1] },
    ]);
  });

  it('ничего не выбрано — сводки нет (CTA погашен)', () => {
    expect(collapsedInviteRows(options, new Set())).toEqual([]);
  });
});

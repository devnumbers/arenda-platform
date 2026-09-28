import { describe, expect, it } from 'vitest';
import {
  acknowledgeFreshFeedEntryIds,
  freshFeedEntryIdsMergedSince,
  noteFreshFeedEntryIds,
} from './live-fresh';

describe('реестр свежих строк ленты (#880: row-in + метка «новое»)', () => {
  it('отмеченные после момента снимка id читаются дельтой', () => {
    noteFreshFeedEntryIds('scope-a', ['c1', 'c2'], 1000);
    expect(freshFeedEntryIdsMergedSince('scope-a', 500)).toStrictEqual(['c1', 'c2']);
    // Снимок краёв ПОСЛЕ влития — дельта пуста.
    expect(freshFeedEntryIdsMergedSince('scope-a', 1000)).toStrictEqual([]);
  });

  it('повторное влитие тех же строк обновляет метку, а не дублирует', () => {
    noteFreshFeedEntryIds('scope-a', ['c1'], 1000);
    noteFreshFeedEntryIds('scope-a', ['c1', 'c3'], 2000);
    expect(freshFeedEntryIdsMergedSince('scope-a', 1500)).toStrictEqual(['c1', 'c3']);
  });

  it('скоупы независимы', () => {
    noteFreshFeedEntryIds('scope-a', ['c1'], 1000);
    expect(freshFeedEntryIdsMergedSince('scope-b', 0)).toStrictEqual([]);
  });

  it('подтверждение читателем гасит реестр скоупа', () => {
    noteFreshFeedEntryIds('scope-a', ['c1'], 1000);
    acknowledgeFreshFeedEntryIds('scope-a');
    expect(freshFeedEntryIdsMergedSince('scope-a', 0)).toStrictEqual([]);
    // Гашение несуществующего скоупа безвредно.
    expect(() => acknowledgeFreshFeedEntryIds('scope-x')).not.toThrow();
  });

  it('пустая порция ничего не пишет', () => {
    noteFreshFeedEntryIds('scope-a', [], 1000);
    expect(freshFeedEntryIdsMergedSince('scope-a', 0)).toStrictEqual([]);
  });
});

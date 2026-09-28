import { describe, expect, it } from 'vitest';
import { circleIconPair, circleIconRing } from './circle-icon-variants';

describe('circleIconRing', () => {
  it('кант 2.5px красится цветом подложки: серая — surface-muted, белая — surface', () => {
    expect(circleIconRing.muted).toBe('shadow-[0_0_0_2.5px_var(--dl-surface-muted)]');
    expect(circleIconRing.white).toBe('shadow-[0_0_0_2.5px_var(--dl-surface)]');
  });
});

describe('circleIconPair', () => {
  it('на серой подложке круг белый с кантом подложки', () => {
    expect(circleIconPair.muted).toBe('bg-surface shadow-[0_0_0_2.5px_var(--dl-surface-muted)]');
  });

  it('на белой подложке круг серый с кантом подложки', () => {
    expect(circleIconPair.white).toBe('bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]');
  });

  it('кант пары берётся из карты колец — вторых ручных копий тени нет', () => {
    expect(circleIconPair.muted).toContain(circleIconRing.muted);
    expect(circleIconPair.white).toContain(circleIconRing.white);
  });
});

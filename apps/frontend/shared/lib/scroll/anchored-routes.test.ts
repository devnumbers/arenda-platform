import { describe, expect, it } from 'vitest';

import { isAnchoredRoute } from './anchored-routes';

describe('isAnchoredRoute — реестр якорёных роутов D8.1', () => {
  it('все три страницы истории — якорёные, включая вложенные', () => {
    expect(isAnchoredRoute('/history')).toBe(true);
    expect(isAnchoredRoute('/history/properties/456')).toBe(true);
    expect(isAnchoredRoute('/history/participants/123/properties/456')).toBe(true);
  });

  it('чужие роуты и похожие префиксы — нет', () => {
    expect(isAnchoredRoute('/history-x')).toBe(false);
    expect(isAnchoredRoute('/operations')).toBe(false);
    expect(isAnchoredRoute('/')).toBe(false);
  });
});

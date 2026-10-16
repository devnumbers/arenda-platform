import { describe, expect, it } from 'vitest';

import { formatPhoneDisplay } from './phone';

describe('formatPhoneDisplay', () => {
  it('канонический +7XXXXXXXXXX — в маску +7 (XXX) XXX-XX-XX', () => {
    expect(formatPhoneDisplay('+79123456789')).toBe('+7 (912) 345-67-89');
  });

  it('маска не меняется (идемпотентно)', () => {
    expect(formatPhoneDisplay('+7 (912) 345-67-89')).toBe('+7 (912) 345-67-89');
  });

  it('не канонический номер возвращается как есть', () => {
    expect(formatPhoneDisplay('')).toBe('');
    expect(formatPhoneDisplay('+7993')).toBe('+7993');
    expect(formatPhoneDisplay('sip:9123456789')).toBe('sip:9123456789');
  });
});

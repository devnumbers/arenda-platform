import { describe, expect, it } from 'vitest';
import {
    formatMoneyKopecks,
    kopecksToAmountInputString,
    kopecksToRublesString,
    parseRublesToKopecks,
    ratioToPercent,
} from './format-money';

describe('formatMoneyKopecks', () => {
    // ru-RU groups digits with a non-breaking space (U+00A0).
    it('formats kopecks as rubles with grouping and the currency symbol', () => {
        expect(formatMoneyKopecks(1234567)).toBe('12\u00A0345,67 ₽');
        expect(formatMoneyKopecks(101)).toBe('1,01 ₽');
    });

    it('rounds to whole rubles on demand', () => {
        expect(formatMoneyKopecks(1234567, {round: true})).toBe('12\u00A0346 ₽');
        expect(formatMoneyKopecks(49, {round: true})).toBe('0 ₽');
        expect(formatMoneyKopecks(50, {round: true})).toBe('1 ₽');
    });

    it('returns an empty string for non-finite input', () => {
        expect(formatMoneyKopecks(Number.NaN)).toBe('');
        expect(formatMoneyKopecks(Number.POSITIVE_INFINITY)).toBe('');
    });
});

describe('kopecksToRublesString', () => {
    it('renders a plain decimal string without grouping or the currency symbol', () => {
        expect(kopecksToRublesString(1234567)).toBe('12345.67');
        expect(kopecksToRublesString(101)).toBe('1.01');
    });

    it('always keeps two decimal places', () => {
        expect(kopecksToRublesString(0)).toBe('0.00');
        expect(kopecksToRublesString(100)).toBe('1.00');
        expect(kopecksToRublesString(1)).toBe('0.01');
    });
});

describe('kopecksToAmountInputString', () => {
    it('whole rubles render without a decimal part', () => {
        expect(kopecksToAmountInputString(14500)).toBe('145');
        expect(kopecksToAmountInputString(250000)).toBe('2500');
    });

    it('fractional amounts render with exactly two decimals', () => {
        expect(kopecksToAmountInputString(14550)).toBe('145,50');
        expect(kopecksToAmountInputString(123450)).toBe('1234,50');
        expect(kopecksToAmountInputString(5)).toBe('0,05');
    });
});

describe('parseRublesToKopecks', () => {
    it('parses dot and comma decimal separators, rounding to the kopeck', () => {
        expect(parseRublesToKopecks('123.45')).toBe(12345);
        expect(parseRublesToKopecks('123,45')).toBe(12345);
        expect(parseRublesToKopecks(' 42 ')).toBe(4200);
        expect(parseRublesToKopecks('0.005')).toBe(1);
        expect(parseRublesToKopecks('0.004')).toBe(0);
    });

    it('treats an empty or blank string as no amount', () => {
        expect(parseRublesToKopecks('')).toBeUndefined();
        expect(parseRublesToKopecks('   ')).toBeUndefined();
    });

    it('rejects non-numeric and negative input', () => {
        expect(parseRublesToKopecks('abc')).toBeUndefined();
        expect(parseRublesToKopecks('-1')).toBeUndefined();
    });

    it('keeps zero as a valid amount by default', () => {
        expect(parseRublesToKopecks('0')).toBe(0);
        expect(parseRublesToKopecks('0.00')).toBe(0);
    });

    it('rejects zero when a positive amount is required', () => {
        expect(parseRublesToKopecks('0', {positive: true})).toBeUndefined();
        expect(parseRublesToKopecks('0.00', {positive: true})).toBeUndefined();
        expect(parseRublesToKopecks('0.01', {positive: true})).toBe(1);
    });
});

describe('ratioToPercent', () => {
    it('scales a 0..1 ratio into percent points', () => {
        expect(ratioToPercent(0)).toBe(0);
        expect(ratioToPercent(0.5)).toBe(50);
        expect(ratioToPercent(1)).toBe(100);
    });

    it('does not clamp — callers own the range', () => {
        expect(ratioToPercent(1.5)).toBe(150);
        expect(ratioToPercent(-0.25)).toBe(-25);
    });
});

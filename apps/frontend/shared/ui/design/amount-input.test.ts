import { describe, expect, it } from 'vitest';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { amountKopecks, groupedAmount, sanitizeAmountInput } from './amount-input';

describe('amount-input', () => {
  it('маска: только цифры и одна запятая, точка нормализуется', () => {
    expect(sanitizeAmountInput('')).toBe('');
    expect(sanitizeAmountInput('2500')).toBe('2500');
    expect(sanitizeAmountInput('25.5')).toBe('25,5');
    expect(sanitizeAmountInput('25,5,7')).toBe('25,57');
    expect(sanitizeAmountInput('12a3в4')).toBe('1234');
    expect(sanitizeAmountInput(' 12-34 ')).toBe('1234');
    expect(sanitizeAmountInput('1 234₽')).toBe('1234');
  });

  it('маска: ведущие нули схлопываются, кроме «0,»', () => {
    expect(sanitizeAmountInput('00')).toBe('0');
    expect(sanitizeAmountInput('01')).toBe('1');
    expect(sanitizeAmountInput('0,5')).toBe('0,5');
    expect(sanitizeAmountInput('0')).toBe('0');
    expect(sanitizeAmountInput(',')).toBe('0,');
    expect(sanitizeAmountInput(',5')).toBe('0,5');
  });

  it('маска: не больше 2 копеечных и 7 рублёвых разрядов', () => {
    expect(sanitizeAmountInput('1,234')).toBe('1,23');
    expect(sanitizeAmountInput('12345678')).toBe('1234567');
    expect(sanitizeAmountInput('1234567,89')).toBe('1234567,89');
  });

  it('копейки: пустота и нули — 0, иначе целые копейки', () => {
    expect(amountKopecks('')).toBe(0);
    expect(amountKopecks('0')).toBe(0);
    expect(amountKopecks('0,')).toBe(0);
    expect(amountKopecks('1')).toBe(100);
    expect(amountKopecks('1,5')).toBe(150);
    expect(amountKopecks('1234,56')).toBe(123456);
  });

  it('группировка в поле: разряды неразрывным пробелом, копейки как набраны', () => {
    expect(groupedAmount('')).toBe('');
    expect(groupedAmount('0')).toBe('0');
    expect(groupedAmount('2500')).toBe(`2\u00A0500`);
    expect(groupedAmount('1234,5')).toBe(`1\u00A0234,5`);
    expect(groupedAmount('0,5')).toBe('0,5');
  });

  it('экранная сумма — formatMoneyKopecks поверх копеек', () => {
    expect(formatMoneyKopecks(amountKopecks('2500'))).toBe(`2\u00A0500 ₽`);
    expect(formatMoneyKopecks(amountKopecks('9999999,99'))).toBe(
      `9\u00A0999\u00A0999,99 ₽`,
    );
  });
});

import { describe, expect, it } from 'vitest';
import { isEmailValid } from './email';

describe('isEmailValid', () => {
  it.each([
    ['admin@example.com', true],
    ['ADMIN@EXAMPLE.COM', true],
    ['ivan+tag@example.co.uk', true],
    ['a@b.c', true],
    ['', false],
    ['admin', false],
    ['admin@', false],
    ['@example.com', false],
    ['admin@example', false],
    ['ad min@example.com', false],
    ['admin@exa mple.com', false],
  ])('%j → %j', (input, expected) => {
    expect(isEmailValid(input)).toBe(expected);
  });
});

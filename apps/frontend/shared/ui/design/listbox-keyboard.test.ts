import { describe, expect, it } from 'vitest';
import { listboxKeyAction } from './listbox-keyboard';

// The key→action map is the pure half of the listbox keyboard support
// (quality bar wave B, jsx-a11y recommended): the DOM half (focus movement)
// lives in the components. ARIA listbox pattern, roving-focus variant.
describe('listboxKeyAction', () => {
    it.each([
        ['Enter', 'select'],
        [' ', 'select'],
        ['Escape', 'close'],
        ['ArrowDown', 'next'],
        ['ArrowUp', 'prev'],
        ['Home', 'first'],
        ['End', 'last'],
    ] as const)('maps %s to %s', (key, action) => {
        expect(listboxKeyAction(key)).toBe(action);
    });

    it.each([
        'a',
        '1',
        'Tab',
        'Shift',
        'Backspace',
        'ArrowLeft',
        'ArrowRight',
        '',
    ])('ignores %s (no listbox semantics)', (key) => {
        expect(listboxKeyAction(key)).toBeNull();
    });
});

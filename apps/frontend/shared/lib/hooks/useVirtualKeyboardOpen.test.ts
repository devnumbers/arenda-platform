import { describe, expect, it } from 'vitest';
import { isEditableTarget, shouldKeyboardCloseOnResize } from './useVirtualKeyboardOpen';

/**
 * Contract of the pure virtual-keyboard classifier (решение владельца
 * 24.10.2026: TabBar не едет с клавиатурой — прячется на время её открытого
 * состояния; StickyBottomBar ведёт себя как прежде). The `useVirtualKeyboardOpen`
 * hook is a thin React wrapper — document-level focusin/focusout plus a
 * window-resize backstop — so the behavioral guarantees are pinned here, on
 * the pure layer. Pure logic only — targets are structural literals, no DOM.
 */

describe('isEditableTarget', () => {
  it.each([{ tagName: 'INPUT' }, { tagName: 'TEXTAREA' }])(
    'native editable control $tagName is editable',
    (target) => {
      expect(isEditableTarget(target)).toBe(true);
    },
  );

  it('contenteditable host is editable', () => {
    expect(isEditableTarget({ tagName: 'DIV', isContentEditable: true })).toBe(true);
  });

  it.each([{ tagName: 'BUTTON' }, { tagName: 'SELECT' }, { tagName: 'DIV' }])(
    'non-editable target %# stays closed',
    (target) => {
      expect(isEditableTarget(target)).toBe(false);
    },
  );

  it('native input is editable even though isContentEditable is false (DOM truth)', () => {
    expect(isEditableTarget({ tagName: 'INPUT', isContentEditable: false })).toBe(true);
  });

  it.each([null, undefined, 'input', 42])('non-element target %# is not editable', (target) => {
    expect(isEditableTarget(target)).toBe(false);
  });
});

describe('shouldKeyboardCloseOnResize', () => {
  it('viewport restored to the pre-open height closes', () => {
    expect(shouldKeyboardCloseOnResize(800, 800)).toBe(true);
  });

  it('viewport grown past the pre-open height closes', () => {
    expect(shouldKeyboardCloseOnResize(800, 900)).toBe(true);
  });

  it.each([{ atOpen: 800, now: 799 }, { atOpen: 800, now: 560 }])(
    'viewport still shrunk ($now < $atOpen) keeps the keyboard state',
    ({ atOpen, now }) => {
      expect(shouldKeyboardCloseOnResize(atOpen, now)).toBe(false);
    },
  );
});

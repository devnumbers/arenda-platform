// Keyboard semantics of a listbox with roving focus (ARIA listbox pattern),
// shared by the option variant of ListRow (its key handling delegates to
// runListboxAction via list-row-keyboard) and the address suggestions in
// properties (the search field enters the list with focusListboxEdge).
// listboxKeyAction is the pure tested core; the DOM half (focus movement and
// key dispatch) lives next to it.

export type ListboxKeyAction =
    | 'select'
    | 'close'
    | 'next'
    | 'prev'
    | 'first'
    | 'last';

const LISTBOX_KEYS: Readonly<Record<string, ListboxKeyAction>> = {
    Enter: 'select',
    ' ': 'select',
    Escape: 'close',
    ArrowDown: 'next',
    ArrowUp: 'prev',
    Home: 'first',
    End: 'last',
};

export function listboxKeyAction(key: string): ListboxKeyAction | null {
    return LISTBOX_KEYS[key] ?? null;
}

/**
 * Moves DOM focus to a sibling `[role="option"]` within the closest
 * `[role="listbox"]`. `next`/`prev` wrap around the list edges. Returns the
 * newly focused element (null when the option set is empty or the target is
 * outside the list), so callers can sync their own active-index state.
 */
export function moveOptionFocus(
    current: HTMLElement,
    action: 'next' | 'prev' | 'first' | 'last',
): HTMLElement | null {
    const listbox = current.closest('[role="listbox"]');
    if (!listbox) {
        return null;
    }
    const items = Array.from(
        listbox.querySelectorAll<HTMLElement>('[role="option"]')
    );
    if (items.length === 0) {
        return null;
    }
    const index = items.indexOf(current);
    const target =
        action === 'first'
            ? items[0]
            : action === 'last'
              ? items[items.length - 1]
              : action === 'next'
                ? items[(index + 1) % items.length]
                : items[(index - 1 + items.length) % items.length];
    if (!target) {
        return null;
    }
    target.focus();
    return target;
}

/** Focus the first/last option of a listbox — the trigger's arrow entry. */
export function focusListboxEdge(
    listbox: HTMLElement | null,
    edge: 'first' | 'last',
): void {
    const options = listbox?.querySelectorAll<HTMLElement>('[role="option"]');
    if (!options || options.length === 0) {
        return;
    }
    (edge === 'first' ? options[0] : options[options.length - 1])?.focus();
}

// The minimal event shape the dispatcher needs — structurally compatible with
// React's KeyboardEvent<HTMLElement>.
export type ListboxKeyboardEvent = {
    readonly key: string;
    readonly currentTarget: HTMLElement;
    preventDefault(): void;
};

/**
 * Dispatches a listbox key event for an option row: Enter/Space select,
 * arrows/Home/End move focus (wrapping), Escape closes. `onClose` is optional
 * — an unhandled Escape keeps bubbling, so a listbox-level listener can own
 * the close (Select closes and refocuses its trigger this way). Returns the
 * newly focused option for arrow moves (null otherwise), so callers can sync
 * their own active-index state.
 */
export function runListboxAction(
    event: ListboxKeyboardEvent,
    handlers: {
        onSelect: () => void;
        onClose?: () => void;
    },
): HTMLElement | null {
    const action = listboxKeyAction(event.key);
    if (action === null) {
        return null;
    }
    event.preventDefault();
    if (action === 'select') {
        handlers.onSelect();
        return null;
    }
    if (action === 'close') {
        handlers.onClose?.();
        return null;
    }
    return moveOptionFocus(event.currentTarget, action);
}

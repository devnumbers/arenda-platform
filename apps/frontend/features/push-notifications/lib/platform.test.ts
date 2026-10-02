import { afterEach, describe, expect, it, vi } from 'vitest';
import { isIosDevice, subscribeToPermissionChanges } from './platform';

// Desktop-mode iPadOS 13+ Safari: the UA is byte-identical to desktop macOS
// Safari, which is the whole reason the Mac+touch signal exists.
const MAC_SAFARI_UA =
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4.1 Safari/605.1.15';
const IPHONE_SAFARI_UA =
    'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1';
const IPAD_CLASSIC_UA =
    'Mozilla/5.0 (iPad; CPU OS 12_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/12.1.2 Mobile/15G77 Safari/604.1';
const WINDOWS_TOUCH_UA =
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36';
const ANDROID_UA =
    'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36';

function stubDevice({
    userAgent,
    platform,
    hasTouch = false,
    hasMsStream = false,
}: {
    userAgent: string;
    platform: string;
    hasTouch?: boolean;
    hasMsStream?: boolean;
}): void {
    // Legacy IE/Edge exposes window.MSStream; its presence is the anti-spoof
    // guard. Every other browser just omits the property.
    vi.stubGlobal('window', hasMsStream ? { MSStream: class MSStream {} } : {});
    vi.stubGlobal('navigator', { userAgent, platform });
    // iOS Safari puts ontouchend on document; desktop browsers have no such property.
    vi.stubGlobal('document', hasTouch ? { ontouchend: null } : {});
}

describe('isIosDevice', () => {
    afterEach(() => {
        vi.unstubAllGlobals();
    });

    it('returns false on the server (no window)', () => {
        expect(isIosDevice()).toBe(false);
    });

    it('detects iPhone Safari through the UA token', () => {
        stubDevice({ userAgent: IPHONE_SAFARI_UA, platform: 'iPhone', hasTouch: true });
        expect(isIosDevice()).toBe(true);
    });

    it('detects classic (pre-13) iPad through the UA token', () => {
        stubDevice({ userAgent: IPAD_CLASSIC_UA, platform: 'iPad', hasTouch: true });
        expect(isIosDevice()).toBe(true);
    });

    it('detects iPadOS 13+ in desktop-spoof mode: Mac signature plus touch', () => {
        stubDevice({ userAgent: MAC_SAFARI_UA, platform: 'MacIntel', hasTouch: true });
        expect(isIosDevice()).toBe(true);
    });

    it('rejects desktop macOS: Mac signature without touch', () => {
        stubDevice({ userAgent: MAC_SAFARI_UA, platform: 'MacIntel' });
        expect(isIosDevice()).toBe(false);
    });

    it('rejects a Windows touchscreen laptop', () => {
        stubDevice({ userAgent: WINDOWS_TOUCH_UA, platform: 'Win32', hasTouch: true });
        expect(isIosDevice()).toBe(false);
    });

    it('rejects Android', () => {
        stubDevice({ userAgent: ANDROID_UA, platform: 'Linux armv8l', hasTouch: true });
        expect(isIosDevice()).toBe(false);
    });

    it('rejects legacy IE/Edge even with iOS UA tokens (MSStream guard)', () => {
        stubDevice({
            userAgent: IPHONE_SAFARI_UA,
            platform: 'iPhone',
            hasTouch: true,
            hasMsStream: true,
        });
        expect(isIosDevice()).toBe(false);
    });
});

type PermissionListener = (event: Event) => void;

/** Fake PermissionStatus: remembers change-listeners, `emit()` fires them. */
function stubPermissionStatus(): {
    readonly addEventListener: ReturnType<typeof vi.fn>;
    readonly removeEventListener: ReturnType<typeof vi.fn>;
    readonly emit: () => void;
} {
    const listeners = new Set<PermissionListener>();
    const status = {
        addEventListener: vi.fn((name: string, listener: PermissionListener) => {
            expect(name).toBe('change');
            listeners.add(listener);
        }),
        removeEventListener: vi.fn((name: string, listener: PermissionListener) => {
            expect(name).toBe('change');
            listeners.delete(listener);
        }),
        emit: () => {
            for (const listener of listeners) listener(new Event('change'));
        },
    };
    vi.stubGlobal('window', {});
    vi.stubGlobal('navigator', {
        permissions: { query: vi.fn().mockResolvedValue(status) },
    });
    return status;
}

describe('subscribeToPermissionChanges — живая проба разрешения (спека #1028 §6)', () => {
    afterEach(() => {
        vi.unstubAllGlobals();
    });

    it('смена разрешения извне дёргает слушателя без перезагрузки', async () => {
        const status = stubPermissionStatus();
        const onChange = vi.fn();
        const unsubscribe = subscribeToPermissionChanges(onChange);
        // wiring асинхронный: query возвращает промис
        await vi.waitFor(() => {
            status.emit();
            expect(onChange).toHaveBeenCalled();
        });
        expect(typeof unsubscribe).toBe('function');
    });

    it('отписка снимает слушателя — событие больше не приходит', async () => {
        const status = stubPermissionStatus();
        const onChange = vi.fn();
        const unsubscribe = subscribeToPermissionChanges(onChange);
        await vi.waitFor(() => expect(status.addEventListener).toHaveBeenCalled());
        unsubscribe();
        status.emit();
        expect(onChange).not.toHaveBeenCalled();
        expect(status.removeEventListener).toHaveBeenCalledWith('change', expect.any(Function));
    });

    it('permissions API недоступен — молчаливый no-op без броска', () => {
        vi.stubGlobal('navigator', {});
        const onChange = vi.fn();
        expect(() => subscribeToPermissionChanges(onChange)()).not.toThrow();
    });

    it('query отклонился (старый браузер) — no-op без броска', async () => {
        vi.stubGlobal('navigator', {
            permissions: { query: vi.fn().mockRejectedValue(new TypeError('not supported')) },
        });
        const onChange = vi.fn();
        const unsubscribe = subscribeToPermissionChanges(onChange);
        expect(typeof unsubscribe).toBe('function');
        await new Promise((resolve) => setTimeout(resolve, 0));
        unsubscribe();
    });

    it('нет window (SSR) — no-op', () => {
        vi.stubGlobal('window', undefined);
        expect(() => subscribeToPermissionChanges(() => {})()).not.toThrow();
    });
});

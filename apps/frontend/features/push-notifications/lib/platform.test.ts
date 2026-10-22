import { afterEach, describe, expect, it, vi } from 'vitest';
import {
    allowsGesturelessPermissionPrompt,
    subscribeToPermissionChanges,
} from './platform';

// Матрица isIosDevice переехала вместе с хелпером в shared/lib/pwa/ios.test.ts
// (12.10.2026): детект платформы стал общим слоем. Здесь остаются только
// живая проба разрешения и её деградации.

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

describe('allowsGesturelessPermissionPrompt — платформа допускает окно без жеста (#1548, вариант (c))', () => {
    afterEach(() => {
        vi.unstubAllGlobals();
    });

    /** Собирает мир детектора: UA + (опционально) touch-события документа —
     * второй сигнал isIosDevice для iPadOS, маскирующегося под macOS. */
    function stubUa(userAgent: string, touch = false): void {
        vi.stubGlobal('window', {});
        vi.stubGlobal('navigator', { userAgent });
        vi.stubGlobal('document', touch ? { ontouchend: null } : {});
    }

    it.each([
        [
            'Safari macOS — WebKit, окно без жеста не показывается и может портить default',
            'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4.1 Safari/605.1.15',
            false,
        ],
        [
            'iPhone Safari — вне PWA пушей нет, в PWA включение только тумблером',
            'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1',
            false,
        ],
        [
            'iPadOS, маскирующийся под macOS Safari (desktop UA + touch) — WebKit',
            'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4.1 Safari/605.1.15',
            true,
        ],
    ])('%s — вызова нет', (_name, ua, touch) => {
        stubUa(ua, touch);
        expect(allowsGesturelessPermissionPrompt()).toBe(false);
    });

    it.each([
        [
            'Chrome desktop (macOS) — окно показывается',
            'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36',
        ],
        [
            'Chrome Android 155+ — неблокирующий баннер',
            'Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Mobile Safari/537.36',
        ],
        [
            'Firefox desktop — окно из жеста показывает, без жеста промис реджектится (ловится тихо)',
            'Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:143.0) Gecko/20100101 Firefox/143.0',
        ],
        [
            'Edge desktop — Chromium',
            'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36 Edg/141.0.0.0',
        ],
    ])('%s — вызов есть', (_name, ua) => {
        stubUa(ua);
        expect(allowsGesturelessPermissionPrompt()).toBe(true);
    });

    it('нет window (SSR) — benign default, вызова нет', () => {
        vi.stubGlobal('window', undefined);
        expect(allowsGesturelessPermissionPrompt()).toBe(false);
    });
});

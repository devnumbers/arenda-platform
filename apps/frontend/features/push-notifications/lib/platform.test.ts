import { afterEach, describe, expect, it, vi } from 'vitest';
import { subscribeToPermissionChanges } from './platform';

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

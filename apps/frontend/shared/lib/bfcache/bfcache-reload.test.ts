import {describe, expect, it, vi} from 'vitest';
import {armBfcacheReload, type BfcacheWindow} from './bfcache-reload';

/** Node-двойник окна: ловит pageshow-слушателя, dispatch вручную —
 * vitest ходит в node-окружении без DOM (vitest.config.mts). */
class FakeBfcacheWindow implements BfcacheWindow {
    readonly location = {reload: vi.fn()};

    private listener: ((event: {persisted: boolean}) => void) | null = null;

    addEventListener(_type: 'pageshow', listener: (event: {persisted: boolean}) => void): void {
        this.listener = listener;
    }

    removeEventListener(_type: 'pageshow', listener: (event: {persisted: boolean}) => void): void {
        if (this.listener === listener) {
            this.listener = null;
        }
    }

    dispatch(event: {persisted: boolean}): void {
        this.listener?.(event);
    }
}

describe('armBfcacheReload', () => {
    it('восстановление из bfcache (persisted) перезагружает документ', () => {
        const win = new FakeBfcacheWindow();

        armBfcacheReload(win);
        win.dispatch({persisted: true});

        expect(win.location.reload).toHaveBeenCalledTimes(1);
    });

    it('обычный pageshow (persisted=false) документ не трогает', () => {
        const win = new FakeBfcacheWindow();

        armBfcacheReload(win);
        win.dispatch({persisted: false});

        expect(win.location.reload).not.toHaveBeenCalled();
    });

    it('disarm снимает слушателя — повторный persisted ничего не делает', () => {
        const win = new FakeBfcacheWindow();

        const disarm = armBfcacheReload(win);
        disarm();
        win.dispatch({persisted: true});

        expect(win.location.reload).not.toHaveBeenCalled();
    });
});

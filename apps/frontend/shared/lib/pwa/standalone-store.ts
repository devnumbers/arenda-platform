/**
 * IndexedDB-backed flag that marks the current browsing context as a PWA
 * (standalone) client. Written by the page, read by the service worker.
 *
 * Why IndexedDB (not postMessage)? The service worker must answer the fetch
 * event synchronously-ish and cannot wait for a round-trip message to the page
 * — especially when the SW itself is what intercepted the navigation.
 * IndexedDB is shared between the page and the SW on the same origin and is
 * readable directly from the SW's fetch handler.
 *
 * The DB name (`rentli-pwa`), object store (`pwa`), and key (`standalone`) are
 * duplicated inline in `public/sw.js` (a static script that cannot import TS).
 * The guard test `standalone-store marker sync with service worker` in
 * `cabinet-routes.test.ts` fails if any of the three markers drift between
 * this file and the SW copy.
 */

export const STANDALONE_DB_NAME = 'rentli-pwa';
export const STANDALONE_DB_STORE = 'pwa';
export const STANDALONE_DB_KEY = 'standalone';

/**
 * Marks the current context as a PWA client. Called after SW registration when
 * `isStandaloneMode()` is true. The flag persists across SW restarts and is
 * read by `sw.js` to redirect PWA navigations to `/` back to `/dashboard`.
 *
 * SSR-safe: no-ops when `indexedDB` is unavailable.
 */
export function markStandaloneClient(): Promise<void> {
    return writeStandaloneFlag(true);
}

function writeStandaloneFlag(value: boolean): Promise<void> {
    if (typeof indexedDB === 'undefined') return Promise.resolve();

    return new Promise((resolve) => {
        let settled = false;
        const done = (): void => {
            if (!settled) {
                settled = true;
                resolve();
            }
        };

        let open: IDBOpenDBRequest;
        try {
            open = indexedDB.open(STANDALONE_DB_NAME, 1);
        } catch {
            done();
            return;
        }

        open.onupgradeneeded = (): void => {
            const db = open.result;
            if (!db.objectStoreNames.contains(STANDALONE_DB_STORE)) {
                db.createObjectStore(STANDALONE_DB_STORE);
            }
        };

        open.onsuccess = (): void => {
            const db = open.result;
            try {
                const tx = db.transaction(STANDALONE_DB_STORE, 'readwrite');
                const store = tx.objectStore(STANDALONE_DB_STORE);
                store.put(value, STANDALONE_DB_KEY);
                tx.oncomplete = (): void => {
                    db.close();
                    done();
                };
                tx.onerror = (): void => {
                    db.close();
                    done();
                };
                tx.onabort = (): void => {
                    db.close();
                    done();
                };
            } catch {
                db.close();
                done();
            }
        };

        open.onerror = done;
    });
}

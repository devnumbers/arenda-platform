import { afterEach, describe, expect, it, vi } from 'vitest';
import { createDraftStore } from './useDraftStore';

/**
 * Contract of the shared form-draft store (spec #378, decision #333 §4).
 * Six `use-*-draft.ts` copies used to reimplement this sessionStorage
 * pattern with a setState-in-effect hydration load; the store replaces the
 * copies, so every behavioral guarantee they relied on is pinned here.
 * Pure logic only — the store is exercised without React, the browser is a
 * stubbed `window.sessionStorage`.
 */

type TestDraft = {
  step: 'form' | 'done';
  note: string;
};

const DEFAULT_DRAFT: TestDraft = { step: 'form', note: '' };

function validateTestDraft(parsed: unknown): TestDraft {
  if (parsed === null || typeof parsed !== 'object') return DEFAULT_DRAFT;
  const record = parsed as Record<string, unknown>;
  if (record.step !== 'form' && record.step !== 'done') return DEFAULT_DRAFT;
  if (typeof record.note !== 'string') return DEFAULT_DRAFT;
  return { step: record.step, note: record.note };
}

type StorageArea = {
  getItem: (key: string) => string | null;
  setItem: (key: string, value: string) => void;
  removeItem: (key: string) => void;
};

function stubSessionStorage(): StorageArea & { read(key: string): string | null } {
  const map = new Map<string, string>();
  const area: StorageArea = {
    getItem: (key) => map.get(key) ?? null,
    setItem: (key, value) => {
      map.set(key, value);
    },
    removeItem: (key) => {
      map.delete(key);
    },
  };
  vi.stubGlobal('window', { sessionStorage: area });
  return { ...area, read: (key) => map.get(key) ?? null };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('createDraftStore server/client snapshots', () => {
  it('getServerSnapshot returns the default with isLoaded=false, identity-stable', () => {
    const store = createDraftStore<TestDraft>({
      storageKey: 'test-draft',
      createDefault: () => DEFAULT_DRAFT,
      validate: validateTestDraft,
    });

    expect(store.getServerSnapshot()).toStrictEqual({ draft: DEFAULT_DRAFT, isLoaded: false });
    // New identity per call would loop useSyncExternalStore re-renders.
    expect(store.getServerSnapshot()).toBe(store.getServerSnapshot());
  });

  it('getSnapshot loads the default when storage is empty and is identity-stable', () => {
    stubSessionStorage();
    const store = createDraftStore<TestDraft>({
      storageKey: 'test-draft',
      createDefault: () => DEFAULT_DRAFT,
      validate: validateTestDraft,
    });

    expect(store.getSnapshot()).toStrictEqual({ draft: DEFAULT_DRAFT, isLoaded: true });
    expect(store.getSnapshot()).toBe(store.getSnapshot());
  });

  it('getSnapshot reads a persisted draft through validate', () => {
    const storage = stubSessionStorage();
    storage.setItem('test-draft', JSON.stringify({ step: 'done', note: 'hi' }));
    const store = createDraftStore<TestDraft>({
      storageKey: 'test-draft',
      createDefault: () => DEFAULT_DRAFT,
      validate: validateTestDraft,
    });

    expect(store.getSnapshot()).toStrictEqual({ draft: { step: 'done', note: 'hi' }, isLoaded: true });
  });

  it('corrupt or invalid persisted JSON falls back to the default', () => {
    const storage = stubSessionStorage();
    storage.setItem('broken-draft', '{not json');
    storage.setItem('invalid-draft', JSON.stringify({ step: 'elsewhere', note: 1 }));
    const broken = createDraftStore<TestDraft>({
      storageKey: 'broken-draft',
      createDefault: () => DEFAULT_DRAFT,
      validate: validateTestDraft,
    });
    const invalid = createDraftStore<TestDraft>({
      storageKey: 'invalid-draft',
      createDefault: () => DEFAULT_DRAFT,
      validate: validateTestDraft,
    });

    expect(broken.getSnapshot().draft).toBe(DEFAULT_DRAFT);
    expect(invalid.getSnapshot().draft).toBe(DEFAULT_DRAFT);
  });

  it('getSnapshot without a browser (SSR) returns the default instead of throwing', () => {
    const store = createDraftStore<TestDraft>({
      storageKey: 'test-draft',
      createDefault: () => DEFAULT_DRAFT,
      validate: validateTestDraft,
    });

    expect(store.getSnapshot()).toStrictEqual({ draft: DEFAULT_DRAFT, isLoaded: true });
  });
});

describe('createDraftStore setDraft (auto persist)', () => {
  it('updates the snapshot, notifies subscribers, and writes storage', () => {
    const storage = stubSessionStorage();
    const store = createDraftStore<TestDraft>({
      storageKey: 'test-draft',
      createDefault: () => DEFAULT_DRAFT,
      validate: validateTestDraft,
    });
    const listener = vi.fn();
    store.subscribe(listener);

    const next: TestDraft = { step: 'form', note: 'edited' };
    store.setDraft(next);

    expect(store.getSnapshot().draft).toBe(next);
    expect(store.getSnapshot().isLoaded).toBe(true);
    expect(listener).toHaveBeenCalledOnce();
    expect(storage.read('test-draft')).toBe(JSON.stringify(next));
  });

  it('passes the previous draft to updater functions', () => {
    const storage = stubSessionStorage();
    const store = createDraftStore<TestDraft>({
      storageKey: 'test-draft',
      createDefault: () => DEFAULT_DRAFT,
      validate: validateTestDraft,
    });
    store.getSnapshot();

    store.setDraft((prev) => ({ ...prev, note: 'from-updater' }));

    expect(store.getSnapshot().draft.note).toBe('from-updater');
    expect(storage.read('test-draft')).toBe(JSON.stringify({ step: 'form', note: 'from-updater' }));
  });

  it('skips storage writes and notifications for an unchanged reference', () => {
    const storage = stubSessionStorage();
    const store = createDraftStore<TestDraft>({
      storageKey: 'test-draft',
      createDefault: () => DEFAULT_DRAFT,
      validate: validateTestDraft,
    });
    store.getSnapshot();
    const listener = vi.fn();
    store.subscribe(listener);

    store.setDraft(DEFAULT_DRAFT);

    expect(listener).not.toHaveBeenCalled();
    expect(storage.read('test-draft')).toBeNull();
  });

  it('removes the draft from storage on a terminal value but keeps it in the snapshot', () => {
    const storage = stubSessionStorage();
    const store = createDraftStore<TestDraft>({
      storageKey: 'test-draft',
      createDefault: () => DEFAULT_DRAFT,
      validate: validateTestDraft,
      isTerminal: (draft) => draft.step === 'done',
    });

    const done: TestDraft = { step: 'done', note: 'finished' };
    store.setDraft(done);

    expect(storage.read('test-draft')).toBeNull();
    expect(store.getSnapshot().draft).toBe(done);
  });

  it('survives a throwing sessionStorage (quota / privacy mode)', () => {
    vi.stubGlobal('window', {
      sessionStorage: {
        getItem: () => null,
        setItem: () => {
          throw new Error('QuotaExceededError');
        },
        removeItem: () => undefined,
      },
    });
    const store = createDraftStore<TestDraft>({
      storageKey: 'test-draft',
      createDefault: () => DEFAULT_DRAFT,
      validate: validateTestDraft,
    });

    expect(() => store.setDraft({ step: 'form', note: 'x' })).not.toThrow();
    expect(store.getSnapshot().draft).toStrictEqual({ step: 'form', note: 'x' });
  });
});

describe('createDraftStore clearDraft', () => {
  it('resets the snapshot to the default, removes storage, and notifies', () => {
    const storage = stubSessionStorage();
    const store = createDraftStore<TestDraft>({
      storageKey: 'test-draft',
      createDefault: () => DEFAULT_DRAFT,
      validate: validateTestDraft,
    });
    store.setDraft({ step: 'form', note: 'typed' });
    expect(storage.read('test-draft')).not.toBeNull();

    const listener = vi.fn();
    store.subscribe(listener);
    store.clearDraft();

    expect(store.getSnapshot().draft).toBe(DEFAULT_DRAFT);
    expect(storage.read('test-draft')).toBeNull();
    expect(listener).toHaveBeenCalledOnce();
  });
});

describe('createDraftStore manual persist', () => {
  it('setDraft never writes storage; saveDraft writes storage without touching the snapshot', () => {
    const storage = stubSessionStorage();
    const store = createDraftStore<TestDraft>({
      storageKey: 'edit-draft',
      createDefault: () => DEFAULT_DRAFT,
      validate: validateTestDraft,
      persist: 'manual',
    });
    const listener = vi.fn();
    store.subscribe(listener);

    store.setDraft({ step: 'form', note: 'in-memory' });
    expect(storage.read('edit-draft')).toBeNull();
    expect(listener).toHaveBeenCalledOnce();

    const saved: TestDraft = { step: 'form', note: 'keystroke' };
    store.saveDraft(saved);
    expect(storage.read('edit-draft')).toBe(JSON.stringify(saved));
    // saveDraft is the write-only seam: no snapshot change, no re-render.
    expect(store.getSnapshot().draft).toStrictEqual({ step: 'form', note: 'in-memory' });
    expect(listener).toHaveBeenCalledOnce();
  });
});

describe('createDraftStore subscribe', () => {
  it('stops delivering notifications after unsubscribe', () => {
    stubSessionStorage();
    const store = createDraftStore<TestDraft>({
      storageKey: 'test-draft',
      createDefault: () => DEFAULT_DRAFT,
      validate: validateTestDraft,
    });
    const listener = vi.fn();
    const unsubscribe = store.subscribe(listener);

    store.setDraft({ step: 'form', note: 'one' });
    unsubscribe();
    store.setDraft({ step: 'form', note: 'two' });

    expect(listener).toHaveBeenCalledOnce();
  });
});

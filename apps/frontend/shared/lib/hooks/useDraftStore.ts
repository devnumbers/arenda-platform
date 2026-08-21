'use client';

import { useState, useSyncExternalStore } from 'react';
import type { Dispatch, SetStateAction } from 'react';

/**
 * Shared form-draft store (decision #333 §4): a sessionStorage-backed draft
 * that loads after hydration and persists on every change. One implementation
 * for every `use-*-draft.ts` wrapper — the wrapper owns only the storage key,
 * the default draft, and the `validate` shape check.
 *
 * Hydration safety comes from `useSyncExternalStore`: the server snapshot is
 * the default draft, and the client snapshot (the persisted draft, read once
 * on the first client access) replaces it after hydration — no setState
 * inside an effect, so no `set-state-in-effect` suppression is ever needed.
 */

export type DraftSnapshot<T> = {
  readonly draft: T;
  readonly isLoaded: boolean;
};

export type DraftStoreConfig<T> = {
  /** sessionStorage key; bound at first render — a wrapper with a dynamic key remounts per key. */
  readonly storageKey: string;
  /** Fallback used before hydration and whenever storage holds no valid draft. */
  readonly createDefault: () => T;
  /** Turns a persisted payload into a draft; returns the default on anything unexpected. */
  readonly validate: (parsed: unknown) => T;
  /** 'auto' (default) persists every change; 'manual' persists only through saveDraft. */
  readonly persist?: 'auto' | 'manual';
  /** auto mode only: terminal drafts (success steps) are removed from storage instead of written. */
  readonly isTerminal?: (draft: T) => boolean;
};

export type SessionDraftStore<T> = {
  readonly subscribe: (listener: () => void) => () => void;
  readonly getSnapshot: () => DraftSnapshot<T>;
  readonly getServerSnapshot: () => DraftSnapshot<T>;
  readonly setDraft: Dispatch<SetStateAction<T>>;
  readonly saveDraft: (draft: T) => void;
  readonly clearDraft: () => void;
};

export function createDraftStore<T>(config: DraftStoreConfig<T>): SessionDraftStore<T> {
  const { storageKey, createDefault, validate } = config;
  const isManual = (config.persist ?? 'auto') === 'manual';
  const isTerminal = config.isTerminal;

  const listeners = new Set<() => void>();
  let serverSnapshot: DraftSnapshot<T> | undefined;
  let clientSnapshot: DraftSnapshot<T> | undefined;

  function readStoredDraft(): T {
    if (typeof window === 'undefined') return createDefault();
    try {
      const raw = window.sessionStorage.getItem(storageKey);
      if (raw === null) return createDefault();
      return validate(JSON.parse(raw) as unknown);
    } catch {
      return createDefault();
    }
  }

  function writeDraft(draft: T): void {
    if (typeof window === 'undefined') return;
    try {
      if (isTerminal !== undefined && isTerminal(draft)) {
        window.sessionStorage.removeItem(storageKey);
        return;
      }
      window.sessionStorage.setItem(storageKey, JSON.stringify(draft));
    } catch {
      // Ignore storage quota / privacy mode errors.
    }
  }

  function removeStoredDraft(): void {
    if (typeof window === 'undefined') return;
    try {
      window.sessionStorage.removeItem(storageKey);
    } catch {
      // Ignore storage errors.
    }
  }

  return {
    subscribe(listener) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    getSnapshot() {
      clientSnapshot ??= { draft: readStoredDraft(), isLoaded: true };
      return clientSnapshot;
    },
    getServerSnapshot() {
      serverSnapshot ??= { draft: createDefault(), isLoaded: false };
      return serverSnapshot;
    },
    setDraft(action) {
      clientSnapshot ??= { draft: readStoredDraft(), isLoaded: true };
      const base = clientSnapshot.draft;
      const next = typeof action === 'function' ? (action as (prev: T) => T)(base) : action;
      if (Object.is(next, base)) return;
      if (!isManual) writeDraft(next);
      clientSnapshot = { draft: next, isLoaded: true };
      listeners.forEach((listener) => listener());
    },
    saveDraft(draft) {
      // Manual-mode write-only seam: storage changes without a snapshot
      // update, so keystroke saves never re-render the form.
      if (typeof window === 'undefined') return;
      try {
        window.sessionStorage.setItem(storageKey, JSON.stringify(draft));
      } catch {
        // Ignore storage quota / privacy mode errors.
      }
    },
    clearDraft() {
      removeStoredDraft();
      const next = createDefault();
      if (clientSnapshot !== undefined && Object.is(next, clientSnapshot.draft)) return;
      clientSnapshot = { draft: next, isLoaded: true };
      listeners.forEach((listener) => listener());
    },
  };
}

export type DraftStore<T> = {
  readonly draft: T;
  readonly isLoaded: boolean;
  readonly setDraft: Dispatch<SetStateAction<T>>;
  readonly saveDraft: (draft: T) => void;
  readonly clearDraft: () => void;
};

export function useDraftStore<T>(config: DraftStoreConfig<T>): DraftStore<T> {
  const [store] = useState(() => createDraftStore(config));
  const { draft, isLoaded } = useSyncExternalStore(
    store.subscribe,
    store.getSnapshot,
    store.getServerSnapshot,
  );
  return { draft, isLoaded, setDraft: store.setDraft, saveDraft: store.saveDraft, clearDraft: store.clearDraft };
}

/** Clears a draft outside a mounted store (e.g. after navigating away mid-flow). */
export function clearDraftStorage(storageKey: string): void {
  if (typeof window === 'undefined') return;
  try {
    window.sessionStorage.removeItem(storageKey);
  } catch {
    // Ignore storage errors.
  }
}

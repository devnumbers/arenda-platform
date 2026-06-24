import { useSyncExternalStore } from 'react';
import { RESEND_TIMEOUT } from './constants';

const STORAGE_KEY = 'arenda:lastSmsSendAt';

type Listener = () => void;

type CooldownStore = {
  subscribe: (listener: Listener) => () => void;
  getSnapshot: () => number;
  getServerSnapshot: () => number;
  recordSend: () => void;
};

function getStoredSendAt(): number | null {
  if (typeof window === 'undefined') {
    return null;
  }

  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) {
    return null;
  }

  const value = Number(raw);
  if (!Number.isFinite(value) || value <= 0) {
    return null;
  }

  return value;
}

function computeRemainingSeconds(sendAt: number | null): number {
  if (sendAt === null) {
    return 0;
  }

  const elapsed = Math.floor((Date.now() - sendAt) / 1000);
  return Math.max(0, Math.min(RESEND_TIMEOUT, RESEND_TIMEOUT - elapsed));
}

function clearStoredSendAt(): void {
  if (typeof window !== 'undefined') {
    window.localStorage.removeItem(STORAGE_KEY);
  }
}

function createCooldownStore(): CooldownStore {
  const listeners = new Set<Listener>();
  let intervalId: number | null = null;

  function notify() {
    listeners.forEach((listener) => listener());
  }

  function handleStorage(event: StorageEvent) {
    if (event.key === STORAGE_KEY) {
      notify();
    }
  }

  function start() {
    if (intervalId !== null) {
      return;
    }

    intervalId = window.setInterval(() => {
      notify();
    }, 1000);

    window.addEventListener('storage', handleStorage);
  }

  function stop() {
    if (intervalId === null) {
      return;
    }

    window.clearInterval(intervalId);
    intervalId = null;
    window.removeEventListener('storage', handleStorage);
  }

  function subscribe(listener: Listener) {
    listeners.add(listener);
    start();

    return () => {
      listeners.delete(listener);
      if (listeners.size === 0) {
        stop();
      }
    };
  }

  function getSnapshot() {
    const remaining = computeRemainingSeconds(getStoredSendAt());

    if (remaining === 0) {
      clearStoredSendAt();
    }

    return remaining;
  }

  function getServerSnapshot() {
    return 0;
  }

  function recordSend() {
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(STORAGE_KEY, String(Date.now()));
    }
    notify();
  }

  return { subscribe, getSnapshot, getServerSnapshot, recordSend };
}

const cooldownStore = createCooldownStore();

export function useSendCooldown() {
  const remainingSeconds = useSyncExternalStore(
    cooldownStore.subscribe,
    cooldownStore.getSnapshot,
    cooldownStore.getServerSnapshot,
  );

  return { remainingSeconds, recordSend: cooldownStore.recordSend };
}

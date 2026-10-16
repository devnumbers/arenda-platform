/**
 * Per-browser flag "the auto-prompt has already been shown" (спека #1028 §3,
 * слайс 2 #1038). Lives in localStorage, not on the backend: the prompt must
 * not chase the user across devices, only across sessions of this browser.
 * The flag is set once, before `requestPermission()` resolves, regardless of
 * the outcome — there are no repeat auto-prompts, ever; turning push on
 * afterwards is always the manual toggle path.
 */

const PUSH_PROMPT_SHOWN_KEY = 'push-prompt-shown';
export { PUSH_PROMPT_SHOWN_KEY };

/**
 * True when this browser has already seen the auto-prompt. An unavailable
 * localStorage (private mode, storage quota) counts as shown: the auto-prompt
 * is an onboarding courtesy, and a browser that cannot remember the flag
 * would be prompted on every load — the one outcome the spec forbids.
 */
export function hasPushPromptBeenShown(): boolean {
  if (typeof window === 'undefined') return false;
  try {
    return window.localStorage.getItem(PUSH_PROMPT_SHOWN_KEY) === '1';
  } catch {
    return true;
  }
}

/** Marks this browser as prompted. Storage errors are swallowed: the flag is
 * best-effort and must never break the load. */
export function markPushPromptShown(): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(PUSH_PROMPT_SHOWN_KEY, '1');
  } catch {
    // Quota/private-mode: nothing to persist. hasPushPromptBeenShown reads a
    // broken store as shown, so the auto-prompt stays silent too — the flag
    // and the read fail closed together.
  }
}

/**
 * Pure push-payload helpers shared between the service worker and unit tests.
 *
 * The service worker (`public/sw.js`) keeps an inline copy of these functions
 * because a static SW cannot import TypeScript at runtime — see the guard test
 * in `push-payload.test.ts` ("service worker handlers stay in sync").
 *
 * The wire payload is a JSON object sent by the backend PushSender:
 *   `{ "title": string, "body": string, "tag"?: string, "url"?: string,
 *      "eventType"?: string }`
 * kept well under the 3993-byte push-message ceiling (RFC 8030).
 */

/** Push notification icon delivered by the SW. Lives in `public/icons/`. */
const PUSH_ICON_PATH = '/icons/icon-192.png';

/** Fallback destination when the payload carries no `url`. */
export const DEFAULT_PUSH_CLICK_URL = '/dashboard';

/** Fallback notification title when the payload omits `title`. */
export const DEFAULT_PUSH_TITLE = 'Рентли';

/** Shape of the decoded backend push payload. */
export type PushPayload = {
    readonly title: string;
    readonly body: string;
    readonly tag: string;
    readonly url: string;
    readonly eventType: string | null;
};

/**
 * Options passed to `registration.showNotification(title, options)`. Mirrors
 * the subset of `NotificationOptions` the SW actually uses.
 */
export type ShowNotificationOptions = {
    readonly body: string;
    readonly tag: string;
    readonly icon: string;
    readonly badge: string;
    readonly data: { readonly url: string };
};

/**
 * Parse the raw `PushEvent.data` text into a {@link PushPayload}.
 *
 * Returns `null` when the payload is absent or not valid JSON — callers
 * (the SW `push` handler) must still show a notification in that case, because
 * Chrome requires every `push` event to surface a visible notification and
 * Safari revokes permission on silent pushes.
 */
export function parsePushPayload(
    eventData: string | undefined | null,
): PushPayload | null {
    if (eventData == null || eventData === '') {
        return null;
    }
    let parsed: unknown;
    try {
        parsed = JSON.parse(eventData);
    } catch {
        return null;
    }
    if (!parsed || typeof parsed !== 'object') {
        return null;
    }
    const record = parsed as Record<string, unknown>;
    const title = typeof record.title === 'string' ? record.title : DEFAULT_PUSH_TITLE;
    const body = typeof record.body === 'string' ? record.body : '';
    const tag = typeof record.tag === 'string' && record.tag.length > 0 ? record.tag : 'rentli-reminder';
    const url = resolveClickTarget(record.url);
    const eventType = typeof record.eventType === 'string' ? record.eventType : null;
    return { title, body, tag, url, eventType };
}

/**
 * Build the `showNotification` options for a parsed payload.
 *
 * `icon`/`badge` point at the branded 192px icon; `tag` enables the browser to
 * replace an existing notification with the same tag (e.g. one per event type)
 * instead of stacking duplicates; `data.url` carries the click destination.
 */
export function buildShowNotificationOptions(
    payload: PushPayload,
): ShowNotificationOptions {
    return {
        body: payload.body,
        tag: payload.tag,
        icon: PUSH_ICON_PATH,
        badge: PUSH_ICON_PATH,
        data: { url: payload.url },
    };
}

/**
 * Resolve a safe click destination from an arbitrary `data.url` value.
 *
 * Only same-origin absolute paths (`/dashboard`, `/properties/123`) are kept.
 * Anything else — missing value, external URLs, `javascript:` schemes, query/
 * hash-only strings — falls back to {@link DEFAULT_PUSH_CLICK_URL}. This stops
 * a malformed or hostile payload from opening an arbitrary page on tap.
 *
 * `unknown` is accepted because the SW reads `event.notification.data`, which
 * is typed loosely by the DOM lib.
 */
export function resolveClickTarget(url: unknown): string {
    if (typeof url !== 'string' || url.length === 0) {
        return DEFAULT_PUSH_CLICK_URL;
    }
    // Reject anything that is not a same-origin absolute path. A leading slash
    // followed by a non-slash character rules out `//host`, `/\\host`, and
    // scheme-relative URLs while allowing real routes.
    if (url[0] !== '/' || url[1] === '/' || url[1] === '\\') {
        return DEFAULT_PUSH_CLICK_URL;
    }
    return url;
}

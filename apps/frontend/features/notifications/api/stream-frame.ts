import { NOTIFICATION_CATEGORIES, type NotificationCategory } from '@/entities/notification';

/**
 * Разбор кадров SSE-стрима (GET /notifications/stream, #742, ADR 0058) на
 * фронте #747: data-строка каждого события — конверт {v, occurredAt,
 * payload}. Кадры best-effort: мусорный или непонятный кадр даёт null и
 * просто игнорируется — лента остаётся системой записи, состояние клиент
 * дочитывает своими запросами.
 */

export type StreamFrame =
  | { readonly kind: 'connected' }
  | {
        readonly kind: 'created';
        readonly id: string;
        readonly category: NotificationCategory;
        /** Строка над заголовком — имя объекта/тарифа или «Системные
         * уведомления»; нет в кадре — строка не рендерится. */
        readonly contextLabel: string | null;
        readonly title: string;
        readonly body: string;
        /** Deeplink события от бэка; в тосте не используется (тап ведёт на
         * страницу уведомления), но валидируется same-origin-гардом. */
        readonly url: string | null;
        readonly occurredAt: string;
    }
  | { readonly kind: 'unread-count'; readonly count: number; readonly occurredAt: string };

/** envelopeVersion бэка (ADR 0058): ломающее изменение payload поднимает v —
 * такой кадр старый клиент игнорирует; аддитивные поля остаются в v1. */
const ENVELOPE_VERSION = 1;

type Envelope = {
  v?: number;
  occurredAt?: string;
  payload?: unknown;
};

type CreatedPayload = {
  id?: string;
  category?: string;
  contextLabel?: string;
  title?: string;
  body?: string;
  url?: string;
};

type UnreadPayload = {
  count?: number;
};

const CATEGORY_SET: ReadonlySet<string> = new Set<string>(NOTIFICATION_CATEGORIES);

/** Same-origin-гард deeplink (канон resolveClickTarget sw.js): только
 * абсолютные пути этого origin — кадр transport-уровня не открывает чужие
 * адреса. */
function sameOriginPath(value: string): boolean {
  if (value.length === 0) return false;
  if (value[0] !== '/' || value[1] === '/' || value[1] === '\\') return false;
  return true;
}

function urlOrNull(value: unknown): string | null {
  const url = text(value);
  return url !== null && sameOriginPath(url) ? url : null;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function text(value: unknown): string | null {
  return typeof value === 'string' && value.length > 0 ? value : null;
}

/**
 * Разбирает пару (имя события, data-строка) в типизированный кадр; null —
 * кадр не наш (неизвестное имя, мусор, ломающая версия).
 */
export function parseStreamFrame(event: string, data: string): StreamFrame | null {
  // Старт-кадр хендлера — служебный «стрим жив», идёт без конверта
  // (Data: "{}", #742): распознаётся по имени до разбора data.
  if (event === 'connected') {
    return { kind: 'connected' };
  }

  let envelope: Envelope;
  try {
    const parsed: unknown = JSON.parse(data);
    if (!isRecord(parsed)) return null;
    envelope = parsed;
  } catch {
    return null;
  }

  if (envelope.v !== ENVELOPE_VERSION) return null;
  const occurredAt = typeof envelope.occurredAt === 'string' ? envelope.occurredAt : '';
  const payload = isRecord(envelope.payload) ? envelope.payload : {};

  if (event === 'notification.created') {
    const created = payload as CreatedPayload;
    const id = text(created.id);
    const title = text(created.title);
    // body и категория необязательны для отрисовки: пустое тело — тост без
    // описания, неизвестная категория деградирует в system (иконка есть —
    // тост не теряем).
    if (id === null || title === null) return null;
    const category =
      typeof created.category === 'string' && CATEGORY_SET.has(created.category)
        ? (created.category as NotificationCategory)
        : 'system';
    return {
      kind: 'created',
      id,
      category,
      contextLabel: text(created.contextLabel),
      title,
      body: typeof created.body === 'string' ? created.body : '',
      url: urlOrNull(created.url),
      occurredAt,
    };
  }

  if (event === 'notification.unread_count') {
    const unread = payload as UnreadPayload;
    if (typeof unread.count !== 'number' || !Number.isFinite(unread.count) || unread.count < 0) {
      return null;
    }
    return { kind: 'unread-count', count: unread.count, occurredAt };
  }

  return null;
}

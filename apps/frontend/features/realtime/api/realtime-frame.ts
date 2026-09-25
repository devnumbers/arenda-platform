/**
 * Разбор кадров SSE-стрима GET /realtime/stream (карта #714, тикет #717;
 * ADR 0062): единственное имя события entity.changed, data-строка — конверт
 * {v, occurredAt, payload} (ADR 0060 §5) с payload {propertyId, entity}.
 * Кадры грубые и best-effort: данных сущности не несут (клиент перечитывает
 * через API), мусорный или непонятный кадр даёт null и просто игнорируется
 * — стрим никогда не становится второй системой правды.
 */

export type RealtimeFrame = {
  /** Имя категории из словаря сущностей (ADR 0062 §2). */
  readonly entity: RealtimeEntity;
  /** Объект изменения; null — безобъектная книга владельца (ADR 0052). */
  readonly propertyId: string | null;
};

/** Словарь сущностей (ADR 0062 §2): стабильные имена категорий — контракт
 * бэк ↔ фронт. Порядок повторяет канонический словарь бэка (domain.All). */
export const REALTIME_ENTITY_NAMES = [
  'payments',
  'operations',
  'tasks',
  'contacts',
  'rentals',
  'property',
  'access',
  'history',
] as const;

export type RealtimeEntity = (typeof REALTIME_ENTITY_NAMES)[number];

const ENTITY_SET: ReadonlySet<string> = new Set<string>(REALTIME_ENTITY_NAMES);

/** envelopeVersion бэка (ADR 0060): ломающее изменение payload поднимает v —
 * такой кадр старый клиент игнорирует; аддитивные поля остаются в v1. */
const ENVELOPE_VERSION = 1;

type Envelope = {
  v?: number;
  payload?: unknown;
};

type Payload = {
  propertyId?: unknown;
  entity?: unknown;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function isRealtimeEntity(value: unknown): value is RealtimeEntity {
  return typeof value === 'string' && ENTITY_SET.has(value);
}

/**
 * Разбирает data-строку кадра entity.changed в типизированный кадр; null —
 * кадр не наш (мусор, ломающая версия, неизвестная сущность).
 */
export function parseRealtimeFrame(data: string): RealtimeFrame | null {
  let envelope: Envelope;
  try {
    const parsed: unknown = JSON.parse(data);
    if (!isRecord(parsed)) return null;
    envelope = parsed;
  } catch {
    return null;
  }

  if (envelope.v !== ENVELOPE_VERSION) return null;
  if (!isRecord(envelope.payload)) return null;
  const payload = envelope.payload as Payload;

  if (!isRealtimeEntity(payload.entity)) return null;
  // propertyId — uuid-строка либо null (безобъектная книга); отсутствующее
  // поле читается как null, а неверного типа кадр — мусорный (null целиком).
  if (payload.propertyId !== undefined && payload.propertyId !== null) {
    if (typeof payload.propertyId !== 'string' || payload.propertyId.length === 0) return null;
  }

  return {
    entity: payload.entity,
    propertyId: typeof payload.propertyId === 'string' ? payload.propertyId : null,
  };
}

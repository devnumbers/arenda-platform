import { runInNewContext } from 'node:vm';
import { describe, expect, it } from 'vitest';
import { BRIDGE_SCRIPT } from './realtime-bridge';

/** Фейк EventSource, каким его видит спека: срез мостового класса. */
type FakeSource = EventTarget & {
  readonly url: string;
  readyState: number;
  close(): void;
};

/** Ставит мост в node-песочнице: BRIDGE_SCRIPT исполняется в изолированном
 * контексте vm c окном-пустышкой; Event/MessageEvent/EventTarget — внешние
 * реальмы (vm-контекст без DOM их не имеет), queueMicrotask — node-глобал. */
function installBridge(): Record<string, unknown> {
  const window: Record<string, unknown> = {};
  runInNewContext(BRIDGE_SCRIPT, { window, Event, MessageEvent, EventTarget, queueMicrotask });
  return window;
}

function createSource(window: Record<string, unknown>, url: string): FakeSource {
  const FakeEventSource = window.EventSource as new (url: string) => FakeSource;
  return new FakeEventSource(url);
}

function emitFrame(window: Record<string, unknown>, data: string): void {
  const emit = window.__emitRealtimeFrame as (data: string) => void;
  emit(data);
}

describe('realtime-bridge — рассылка кадров', () => {
  it('открытые источники получают кадр entity.changed с data-строкой', async () => {
    const window = installBridge();
    const a = createSource(window, '/api/realtime/stream');
    const b = createSource(window, '/api/realtime/stream');
    await Promise.resolve(); // конструктор открывает источник в микротаске
    expect(a.readyState).toBe(1);
    expect(b.readyState).toBe(1);

    const seen: string[] = [];
    a.addEventListener('entity.changed', (event) => {
      seen.push((event as MessageEvent).data as string);
    });
    b.addEventListener('entity.changed', (event) => {
      seen.push((event as MessageEvent).data as string);
    });

    emitFrame(window, '{"v":1}');
    expect(seen).toEqual(['{"v":1}', '{"v":1}']);
  });

  it('close() убирает источник из рассылки — кадры в закрытое соединение не диспатчатся', async () => {
    const window = installBridge();
    const live = createSource(window, '/api/realtime/stream');
    const closed = createSource(window, '/api/realtime/stream');
    await Promise.resolve();

    const seen: string[] = [];
    closed.addEventListener('entity.changed', (event) => {
      seen.push((event as MessageEvent).data as string);
    });
    live.addEventListener('entity.changed', (event) => {
      seen.push((event as MessageEvent).data as string);
    });

    closed.close();
    expect(closed.readyState).toBe(2);

    emitFrame(window, '{"v":1}');
    expect(seen).toEqual(['{"v":1}']);
  });
});

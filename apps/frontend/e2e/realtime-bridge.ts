import type { Page } from '@playwright/test';

/**
 * Тестовый мост realtime-стрима для e2e (карта #714, тикет #718): подменяет
 * браузерный EventSource фейком, который мгновенно «открывается», и даёт
 * спеке ручку window.__emitRealtimeFrame — диспатч кадра entity.changed во
 * все живые источники. Спека сеет строки журнала прямым INSERT'ом (мимо
 * рекордера — кадров бэк не шлёт) и сама диспатчит кадр, как это сделал бы
 * реальный стрим после мутации; чтение ленты при этом идёт через живой бэк.
 * Мост ставится addInitScript'ом ДО навигации — бандл приложения получает
 * фейк вместо настоящего EventSource и реальный SSE не открывается.
 */
/** Тело моста одной строкой-скриптом: исполняется в браузере через
 * addInitScript и в node-тестах ниже (e2e/realtime-bridge.test.ts). */
export const BRIDGE_SCRIPT = `
  (() => {
    const sources = [];
    class FakeEventSource extends EventTarget {
      constructor(url) {
        super();
        this.url = url;
        this.readyState = 0;
        sources.push(this);
        queueMicrotask(() => {
          this.readyState = 1;
          this.dispatchEvent(new Event('open'));
        });
      }
      close() {
        this.readyState = 2;
        // Живой EventSource после close() событий не шлёт — как и гвард
        // видимости sse-client (скрытая вкладка закрывает источник),
        // выпадаем из рассылки __emitRealtimeFrame.
        const i = sources.indexOf(this);
        if (i !== -1) sources.splice(i, 1);
      }
    }
    window.EventSource = FakeEventSource;
    window.__emitRealtimeFrame = (data) => {
      for (const source of sources) {
        source.dispatchEvent(new MessageEvent('entity.changed', { data }));
      }
    };
  })();
`;

export async function installRealtimeBridge(page: Page): Promise<void> {
  await page.addInitScript(BRIDGE_SCRIPT);
}

type RealtimeTestFrame = {
  readonly entity: string;
  readonly propertyId: string | null;
};

/** Диспатчит кадр entity.changed в конверте v1 (ADR 0060 §5) — тот же
 * payload, что шлёт реальный стрим. */
export async function emitRealtimeFrame(page: Page, frame: RealtimeTestFrame): Promise<void> {
  await page.evaluate((payload) => {
    const emit = (window as unknown as { __emitRealtimeFrame?: (data: string) => void })
      .__emitRealtimeFrame;
    if (emit === undefined) {
      throw new Error('realtime bridge is not installed — call installRealtimeBridge first');
    }
    emit(JSON.stringify({ v: 1, occurredAt: new Date().toISOString(), payload }));
  }, frame);
}

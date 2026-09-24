import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { SUPPORT_TELEGRAM_URL } from './support';

// Офлайн-страница (тикет #783) самодостаточна: она отдаётся из кэша SW при
// выключенной сети и не может импортировать TS, поэтому ссылка поддержки из
// support.ts продублирована в её разметке вручную (решение владельца 21.09:
// «Обратиться в поддержку» — прямая ссылка на Telegram, без блока
// копирования). Тест ловит дрейф: поменялся адрес здесь — офлайн-страница
// обязана обновиться тем же коммитом (паттерн guard-тестов public/sw.js,
// см. push-payload.test.ts).
const offlineHtml = readFileSync(resolve(process.cwd(), 'public/offline.html'), 'utf8');

describe('offline page support link stays in sync', () => {
    it('public/offline.html carries the Telegram url', () => {
        expect(offlineHtml).toContain(SUPPORT_TELEGRAM_URL);
    });
});

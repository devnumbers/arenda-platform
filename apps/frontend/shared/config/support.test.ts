import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { SUPPORT_EMAIL, SUPPORT_TELEGRAM_URL } from './support';

// Офлайн-страница (тикет #783) самодостаточна: она отдаётся из кэша SW при
// выключенной сети и не может импортировать TS, поэтому контакты из
// support.ts продублированы в её разметке вручную. Тест ловит дрейф:
// поменяли контакт здесь — офлайн-страница обязана обновиться тем же
// коммитом (паттерн guard-тестов public/sw.js, см. push-payload.test.ts).
const offlineHtml = readFileSync(resolve(process.cwd(), 'public/offline.html'), 'utf8');

describe('offline page support contacts stay in sync', () => {
    it('public/offline.html carries the support email', () => {
        expect(offlineHtml).toContain(SUPPORT_EMAIL);
    });

    it('public/offline.html carries the Telegram url', () => {
        expect(offlineHtml).toContain(SUPPORT_TELEGRAM_URL);
    });

    it('public/offline.html carries the canonical copied hint (канон #766)', () => {
        expect(offlineHtml).toContain('Скопировано');
    });
});

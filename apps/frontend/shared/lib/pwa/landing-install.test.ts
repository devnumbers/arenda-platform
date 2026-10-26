import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

/**
 * Guard-тест установочной поверхности лендинга (карта #1610, research #1612).
 *
 * Установка PWA с лендинга держится на трёх вещах в layout лендинга: линк на
 * манифест кабинета (тот же app id «/» — одна инсталляция), appleWebApp-меты
 * (имя «Рентли» под иконкой на iOS) и инлайн-гард изоляции (standalone-клиент
 * не должен видеть лендинг). Файлы читаются кросс-аппом — канон robots.txt
 * sync-теста; сам лендинг тест-раннера не имеет.
 */
const landingLayoutPath = resolve(process.cwd(), '../landing/app/layout.tsx');

describe('landing install surface sync (карта #1610)', () => {
  const layoutSource = readFileSync(landingLayoutPath, 'utf8');

  it('landing links the cabinet web app manifest', () => {
    // Линк на /manifest.webmanifest кабинета: тот же id «/», start_url
    // /properties — установка с лендинга и из кабинета дают одно приложение.
    // Двойные кавычки — стиль лендинга (Prettier), не опечатка.
    expect(layoutSource).toContain('manifest: "/manifest.webmanifest"');
  });

  it('landing declares the apple web app title «Рентли»', () => {
    // iOS: имя под иконкой домашнего экрана при «Добавить на экран Домой».
    expect(layoutSource).toContain('appleWebApp');
    expect(layoutSource).toContain('"Рентли"');
  });

  it('landing carries the standalone isolation guard', () => {
    // Гард изоляции: в standalone весь лендинг уводит в кабинет до первой
    // отрисовки. Приоритет navigator.standalone на WebKit (известны ложные
    // срабатывания matchMedia — w3c/manifest#1092), matchMedia доверяем
    // только вне WebKit, где standalone === undefined.
    expect(layoutSource).toContain("location.replace('/properties')");
    expect(layoutSource).toContain('navigator.standalone');
    expect(layoutSource).toContain("'(display-mode: standalone)'");
  });
});

import type { Locator, Page } from '@playwright/test';
import { expect, openCabinetWithSessionToken, test } from './fixtures';

// Колесо-«крутилка» дизайн-слоя (решения владельца 2026-10-13): месяцы —
// конечная лента (отмена #810 в части месяцев, часы/минуты остаются
// кольцом), годы без границ — от текущего года вперёд, страница под
// открытым пикером не прокручивается (лок + overscroll-contain), жест
// внутри колеса не тащит vaul-шит, ряды у краёв сжаты «барабаном»,
// край линейной ленты недостижим одним жестом (зона триггера 80 рядов,
// достройка назад — в паузе). Поверхность — витрина /ui-kit: живой
// каталог канона, сидовая сессия нужна только чтобы middleware пустил
// в кабинет.

const UI_KIT = '/ui-kit';

/** Колесо устоялось: два замера scrollTop с шагом 200мс дают одно значение.
 * Анимация колёсного скролла на 3000px живёт ~700мс, сеттл-коммит и
 * отложенная достройка приезжают следом — флики и ассерты имеют смысл
 * только после тишины (клавиатурный подскролл поверх живой анимации и
 * вовсе дожимается ею обратно к целевому ряду). */
async function wheelStill(page: Page, list: Locator): Promise<void> {
  await expect
    .poll(async () => {
      const a = await list.evaluate((el) => Math.round(el.scrollTop));
      await page.waitForTimeout(200);
      const b = await list.evaluate((el) => Math.round(el.scrollTop));
      return a === b;
    }, { timeout: 10_000 })
    .toBe(true);
}

test.beforeEach(async ({ page, seededUser }) => {
  await openCabinetWithSessionToken(page, seededUser.sessionToken);
});

test.describe('WheelPicker: конечные месяцы и годы от текущего', () => {
  test('месяцы — конечная лента: декабрь не докручивается в январь', async ({ page }) => {
    await page.goto(UI_KIT);
    await page.getByRole('button', { name: 'Август, 2026' }).click();
    const monthList = page.getByRole('listbox', { name: 'Месяц' });
    await expect(monthList).toBeVisible();
    await monthList.focus();
    await page.keyboard.press('End'); // декабрь — последний ряд
    await page.keyboard.press('ArrowDown'); // кольцо увезло бы в январь
    await expect(monthList.locator('[aria-selected="true"]')).toHaveText('Декабрь');
  });

  test('годы без границ — от текущего года вперёд, назад не удлиняются', async ({ page }) => {
    // Пол ленты — min(текущий, выбранный в витрине 2026): выбранный год
    // всегда внутри ленты (years-window), сезонно-независимое ожидание.
    const floorYear = String(Math.min(new Date().getFullYear(), 2026));
    await page.goto(UI_KIT);
    await page.getByRole('button', { name: 'Август, 2026' }).click();
    const yearList = page.getByRole('listbox', { name: 'Год' });
    await expect(yearList).toBeVisible();
    // Первый ряд ленты — пол; у верхнего края лента не удлиняется
    // в прошлое (включая после плавной докрутки Home и оседания на краю).
    await yearList.focus();
    await page.keyboard.press('Home');
    await page.keyboard.press('ArrowUp');
    await expect
      .poll(() => yearList.locator('li').first().textContent(), { timeout: 3_000 })
      .toBe(floorYear);
    await expect(yearList.locator('[aria-selected="true"]')).toHaveText(floorYear);
  });

  test('жёсткий флик года вперёд не откатывает колесо к выбранному году', async ({ page }) => {
    await page.goto(UI_KIT);
    await page.getByRole('button', { name: 'Август, 2026' }).click();
    const yearList = page.getByRole('listbox', { name: 'Год' });
    await expect(yearList).toBeVisible();
    await page.waitForTimeout(600); // мгновенная посадка на стартовый год
    const initial = await yearList.locator('[aria-selected="true"]').textContent();
    const initialYear = Number(initial);
    const box = await yearList.boundingBox();
    if (box === null) {
      throw new Error('колесо годов не найдено');
    }
    // Жёсткий флик одним жестом через край стартовой ленты: рост ленты
    // (onNearEnd) не должен синхронизировать позицию колеса — иначе
    // smooth-скролл к выбранному ряду дёргает колесо назад посреди
    // инерции (жалоба владельца 13.10 на пикере создания платежа).
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.wheel(0, 3000);
    await page.waitForTimeout(1_500);
    const settled = await yearList.locator('[aria-selected="true"]').textContent();
    expect(Number(settled)).toBeGreaterThan(initialYear + 20);
  });
});

test.describe('WheelPicker: край ленты недостижим одним жестом (#1427)', () => {
  test('жёсткий флик года вперёд пролетает край стартовой ленты без остановки', async ({ page }) => {
    await page.goto(UI_KIT);
    await page.getByRole('button', { name: 'Август, 2026' }).click();
    const yearList = page.getByRole('listbox', { name: 'Год' });
    await expect(yearList).toBeVisible();
    // Стена стартовой ленты — её последний ряд (выбранный +25). Ставим
    // колесо в два ряда до неё: прежний триггер удлинения сидел за 2 ряда
    // — инерция пролетала их за ~12мс, React не успевал достроить ленту,
    // прокрутка гасла о физический край и «просто останавливалась».
    const wallYear = Math.max(new Date().getFullYear(), 2026) + 25;
    await yearList.focus();
    await page.keyboard.press('End');
    await page.keyboard.press('ArrowUp');
    await page.keyboard.press('ArrowUp');
    // Клавиатурная докрутка анимируется: флик поверх живой программы
    // дожимается ею обратно к целевому ряду — ждём тишины колеса.
    await wheelStill(page, yearList);
    const box = await yearList.boundingBox();
    if (box === null) {
      throw new Error('колесо годов не найдено');
    }
    // Один жёсткий флик (3000px ≈ 62 ряда) из зоны триггера: стена обязана
    // уехать раньше, чем инерция до неё долетит.
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.wheel(0, 3000);
    await wheelStill(page, yearList);
    await page.waitForTimeout(300); // сеттл-коммит после тишины
    const settled = await yearList.locator('[aria-selected="true"]').textContent();
    expect(Number(settled)).toBeGreaterThan(wallYear + 19);
  });

  test('лента годов периода уходит в прошлое без стены и достраивается в паузе', async ({ page }) => {
    // Мобильный вьюпорт: шит колёс — vaul-дроп (на десктопе карточка без
    // входной анимации), флик живёт на телефоне.
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(UI_KIT);
    // Секция «назад без предела»: только max (сентябрь 2026), чип «Июнь,
    // 2026» — конфигурация истории операций (решение владельца 2026-09-04).
    await page.getByRole('button', { name: 'Июнь, 2026' }).click();
    const yearList = page.getByRole('listbox', { name: 'Год' });
    await expect(yearList).toBeVisible();
    // Шит въезжает снизу полсекунды — ждём оседания, как в vaul-тесте ниже.
    await expect
      .poll(() => page.evaluate(() => {
        const drawer = document.querySelector('[data-vaul-drawer]');
        return drawer === null ? '' : getComputedStyle(drawer).transform;
      }), { timeout: 5_000 })
      .toBe('none');
    // Стена в прошлом сеется на глубину первого флика (134 = зона 80 +
    // макс. флик 54, план 048 — посев 400 открывал шит секунду), первый
    // же флик назад встречает ленту, а не «выбранный − 3» (2023).
    const seededFloor = 2026 - 134;
    await expect(yearList.locator('li').first()).toHaveText(String(seededFloor));
    const box = await yearList.boundingBox();
    if (box === null) {
      throw new Error('колесо годов не найдено');
    }
    const cx = box.x + box.width / 2;
    const cy = box.y + box.height / 2;
    // Серия жёстких фликов назад (по ~62 ряда): ни один не упирается в
    // стену — колесо каждый раз уезжает на десятки лет в прошлое. Между
    // фликами ждём тишины: сеттл-коммит и достройка успевают, анимация
    // одного флика не обрывается следующим.
    for (let i = 0; i < 6; i += 1) {
      await page.mouse.move(cx, cy);
      await page.mouse.wheel(0, -3000);
      await wheelStill(page, yearList);
    }
    // Сеттл-коммит последнего флика (и отложенная достройка в том же
    // колбэке) — читаем с поллингом.
    await expect
      .poll(async () =>
        Number(await yearList.locator('[aria-selected="true"]').textContent()),
      { timeout: 5_000 })
      .toBeLessThanOrEqual(2026 - 300);
    // Достройка назад пишется в паузе после остановки: осели в зоне
    // триггера — первый ряд ленты уехал в прошлое глубже посева.
    await expect
      .poll(
        async () =>
          Number(await yearList.locator('li').first().textContent()),
        { timeout: 5_000 },
      )
      .toBeLessThan(seededFloor);
  });

  test('доращивка назад не уезжает с закоммиченного года ни в одном кадре (план 049)', async ({ page }) => {
    // Репро 17.10 (WebKit, движок телефона владельца): после приписки
    // рядов сверху позиция держалась на «реснапе до пейнта», которого
    // в WebKit нет — в полосе ~13 кадров висел год на 400 раньше
    // закоммиченного, затем коррекция катила роллом через века. Проба
    // журналит закрашиваемое состояние (ряд в полосе + длина ленты) и
    // требует: в каждом кадре ПОСЛЕ доращивки (лента выросла) год в
    // полосе — закоммиченный. На Chromium проба зелёная и до фикса
    // (реснап корректен), ловит она именно WebKit — проект webkit.
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(UI_KIT);
    await page.getByRole('button', { name: 'Июнь, 2026' }).click();
    const yearList = page.getByRole('listbox', { name: 'Год' });
    await expect(yearList).toBeVisible();
    await expect
      .poll(() => page.evaluate(() => {
        const drawer = document.querySelector('[data-vaul-drawer]');
        return drawer === null ? '' : getComputedStyle(drawer).transform;
      }), { timeout: 5_000 })
      .toBe('none');
    const before = await yearList.locator('li').count();
    // Журнал закрашиваемых кадров (ряд в полосе, длина ленты).
    await yearList.evaluate((ul) => {
      const w = window as unknown as { __bandJournal: Array<{ year: string; rows: number }> };
      w.__bandJournal = [];
      const tick = () => {
        const rows = ul.children.length;
        const row = Math.round(ul.scrollTop / 48);
        const li = ul.querySelector(`li[data-index="${row}"]`);
        w.__bandJournal.push({ year: li?.textContent ?? '?', rows });
        requestAnimationFrame(tick);
      };
      requestAnimationFrame(tick);
    });
    // Один жёсткий флик назад из зоны посева: settle коммитит смену года
    // и доращивку одним рендером — тот самый совпадение из репро.
    const box = await yearList.boundingBox();
    if (box === null) {
      throw new Error('колесо годов не найдено');
    }
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.wheel(0, -3000);
    await wheelStill(page, yearList);
    // Доращивка доехала: первый ряд ленты уехал в прошлое глубже посева.
    await expect
      .poll(async () => await yearList.locator('li').count(), { timeout: 5_000 })
      .toBeGreaterThan(before);
    await page.waitForTimeout(400); // хвост коррекций после роста ленты
    const committed = await yearList.locator('[aria-selected="true"]').textContent();
    const journal = await page.evaluate(
      () => (window as unknown as { __bandJournal: Array<{ year: string; rows: number }> })
        .__bandJournal,
    );
    const grown = journal.filter((f) => f.rows > before);
    // Проба поймала доращивку — иначе ассерт ниже пустой.
    expect(grown.length).toBeGreaterThan(0);
    for (const frame of grown) {
      expect(frame.year, `кадр после доращивки: ${JSON.stringify(frame)}`).toBe(committed);
    }
  });
});

test.describe('WheelPicker: страница под открытым пикером не прокручивается', () => {
  /** Фон-прокрутка до устойчивого значения: сразу после goto гидрационный
   * свап RSC-дерева может сбросить прокрутку в ноль — крутим, пока не
   * прилипнет (окно свапа — те же ~90–350мс, что в обёртке goto #698). */
  async function stickyScroll(page: Page, y: number): Promise<number> {
    await expect.poll(async () => {
      await page.evaluate((top) => window.scrollTo(0, top), y);
      return page.evaluate(() => window.scrollY);
    }, { timeout: 10_000 }).toBe(y);
    return y;
  }

  test('пикер даты блокирует прокрутку страницы', async ({ page }) => {
    await page.goto(UI_KIT);
    await stickyScroll(page, 2000);
    // Клик автоскроллит кнопку в вьюпорт — базлайн читаем ПОСЛЕ открытия,
    // когда фон заведомо прокручен и пикер уже накрыл страницу.
    await page.getByRole('button', { name: 'Открыть пикер даты' }).click();
    const dialog = page.getByRole('dialog', { name: 'Выбрать дату' });
    await expect(dialog).toBeVisible();
    const scrolled = await page.evaluate(() => window.scrollY);
    expect(scrolled).toBeGreaterThan(0);
    // Жёст у верхней кромки ленты (лента открыта на «сегодня», scrollTop 0):
    // раньше цепочка скролла уезжала в страницу под пикером.
    await page.mouse.move(400, 400);
    await page.mouse.wheel(0, -800);
    const after = await page.evaluate(() => window.scrollY);
    expect(after).toBe(scrolled);
  });

  test('пикер даты блокирует прокрутку страницы на мобиле (тач-цепочка)', async ({ page }) => {
    // Свайп гонится CDP-тачем — движок только у Chromium; тач-семантика на
    // другом движке покрыта живой приёмкой (план 049: webkit-проект —
    // про позицию колеса, не про тач-цепочку).
    test.skip(
      test.info().project.name === 'webkit',
      'CDP-тач недоступен вне Chromium; webkit-проект гоняет позиционные пробы',
    );
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto(UI_KIT);
    await stickyScroll(page, 2000);
    await page.getByRole('button', { name: 'Открыть пикер даты' }).click();
    const dialog = page.getByRole('dialog', { name: 'Выбрать дату' });
    await expect(dialog).toBeVisible();
    const scrolled = await page.evaluate(() => window.scrollY);
    expect(scrolled).toBeGreaterThan(0);
    // Настоящий тач-свайп у верхней кромки ленты (лента на «сегодня», верх):
    // исходная жалоба владельца — тач-цепочка уезжала в страницу под пикером.
    const feed = dialog.locator('.overflow-y-auto');
    const box = await feed.boundingBox();
    if (box === null) {
      throw new Error('лента пикера даты не найдена');
    }
    const cdp = await page.context().newCDPSession(page);
    const x = box.x + box.width / 2;
    await cdp.send('Input.dispatchTouchEvent', {
      type: 'touchStart',
      touchPoints: [{ x, y: box.y + 150 }],
    });
    for (const step of [130, 110, 90, 70]) {
      await cdp.send('Input.dispatchTouchEvent', {
        type: 'touchMove',
        touchPoints: [{ x, y: box.y + step }],
      });
    }
    await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
    await page.waitForTimeout(400);
    const after = await page.evaluate(() => window.scrollY);
    expect(after).toBe(scrolled);
  });

  test('пикер периода блокирует прокрутку страницы', async ({ page }) => {
    await page.goto(UI_KIT);
    await stickyScroll(page, 2000);
    await page.getByRole('button', { name: 'Открыть пикер периода' }).click();
    const dialog = page.getByRole('dialog', { name: 'Выберите период' });
    await expect(dialog).toBeVisible();
    const scrolled = await page.evaluate(() => window.scrollY);
    expect(scrolled).toBeGreaterThan(0);
    // Лента открывается у текущего месяца (нижняя часть окна); доезжаем
    // до нижнего края и давим дальше — назад в страницу уезжать нечему.
    await page.mouse.move(400, 400);
    await page.mouse.wheel(0, 50_000);
    await page.mouse.wheel(0, 800);
    const after = await page.evaluate(() => window.scrollY);
    expect(after).toBe(scrolled);
  });
});

test.describe('WheelPicker: жест колеса не тащит vaul-шит', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('свайп вниз по колесу годов у верхнего края не закрывает шит', async ({ page }) => {
    await page.goto(UI_KIT);
    // Ограниченная секция: лента годов [2026..2026] стоит на верхнем крае —
    // ровно та геометрия, на которой vaul отбирал жест у колеса.
    await page.getByRole('button', { name: 'Сентябрь, 2026' }).click();
    const yearList = page.getByRole('listbox', { name: 'Год' });
    await expect(yearList).toBeVisible();
    // Шит въезжает снизу полсекунды: драг по «плавающему» боксу стартует
    // за нижним краем вьюпорта (в оверлей) и vaul закрывает шит — тянем
    // только после оседания (vaul снимает transform по завершении входа).
    await expect
      .poll(() => page.evaluate(() => {
        const drawer = document.querySelector('[data-vaul-drawer]');
        return drawer === null ? '' : getComputedStyle(drawer).transform;
      }), { timeout: 5_000 })
      .toBe('none');
    const box = await yearList.boundingBox();
    if (box === null) {
      throw new Error('колесо годов не найдено');
    }
    await page.mouse.move(box.x + box.width / 2, box.y + 40);
    await page.mouse.down();
    await page.mouse.move(box.x + box.width / 2, box.y + 320, { steps: 12 });
    await page.mouse.up();
    await expect(yearList).toBeVisible();
    // Пути закрытия не сломались: свайп вниз за ручку (вне колёс) закрывает.
    const sheet = page.locator('[data-vaul-drawer]');
    const sbox = await sheet.boundingBox();
    if (sbox === null) {
      throw new Error('шит не найден');
    }
    await page.mouse.move(sbox.x + sbox.width / 2, sbox.y + 12);
    await page.mouse.down();
    await page.mouse.move(sbox.x + sbox.width / 2, sbox.y + 500, { steps: 14 });
    await page.mouse.up();
    await expect(yearList).toBeHidden();
  });
});

test.describe('WheelPicker: барабан-каскад рядов', () => {
  test('ряды у краёв сжаты и приглушены, центральный — в полную силу', async ({ page }) => {
    await page.goto(UI_KIT);
    await page.getByRole('button', { name: 'Открыть шит колёс' }).click();
    const minuteList = page.getByRole('listbox', { name: 'Минуты' });
    await expect(minuteList).toBeVisible();
    const edgeScale = await minuteList
      .locator('li')
      .first()
      .evaluate((el) => getComputedStyle(el).scale);
    expect(edgeScale).not.toBe('none');
    expect(parseFloat(edgeScale)).toBeLessThan(1);
    const centerScale = await minuteList
      .locator('[aria-selected="true"]')
      .evaluate((el) => getComputedStyle(el).scale);
    expect(parseFloat(centerScale)).toBeCloseTo(1, 2);
  });
});

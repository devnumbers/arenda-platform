import type { Locator, Page, TestInfo } from '@playwright/test';
import {
  captureScreen,
  expect,
  openCabinetWithSessionToken,
  openCabinetWithSeededSession,
  seededMemberSessionToken,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
  todayIso,
} from './fixtures';

// Удаление правила задачи со страницы «Изменить задачу» (#497 — первый вью
// готового контракта DELETE): иконка корзины в правом слоте хедера, без
// подтверждения — сразу (решение владельца 2026-10-22). Семантика —
// глоссарий «Удалённое правило» (#496): жёсткое, невыполненные вхождения
// сносятся, журнал выполненных остаётся (rule_id у строк обнуляется).
// После удаления — goBack на список (канон удаления сущности), тост
// «Задача удалена» живёт в глобальном портале и виден уже на списке.
//
// Правила одноразовые, создаются API-вызовом от живой сессии (канон
// tasks-feed-empty-filter / payment-subscreens: сид не мутируем). Оба
// среза — объектный и без объекта (ADR 0052), плюс полный доступ (D9
// #1309: удаление едет правом редактирования). В правку заходим честным
// путём — тапом по строке списка: goBack идёт по истории, у прямого goto
// её нет (прецедент теста отмены payment-edit-delete).

const APARTMENT = SEEDED_APARTMENT_PROPERTY_ID;

type RuleResponse = { id: string };

/** Создание правила задачи API-вызовом от текущей сессии страницы. */
async function createRule(
  page: Page,
  data: Record<string, unknown>,
  path = `/api/properties/${APARTMENT}/tasks/rules`,
): Promise<string> {
  const response = await page.request.post(path, { data });
  expect(response.ok(), `создание правила ${JSON.stringify(data)}`).toBe(true);
  const rule = (await response.json()) as RuleResponse;
  return rule.id;
}

/** Честный вход в правку: тап по строке задачи на списке объекта. */
async function openEditFromList(page: Page, title: string, ruleId: string): Promise<void> {
  await page.goto(`/properties/${APARTMENT}/tasks`);
  await page.getByText(title).click();
  await expect(page).toHaveURL(new RegExp(`/tasks/${ruleId}/edit$`));
}

/** Счётчик DELETE-запросов по подстроке URL — канон double-submit-guard. */
function trackDeletes(page: Page, urlPart: string): string[] {
  const urls: string[] = [];
  page.on('request', (request) => {
    if (request.method() === 'DELETE' && request.url().includes(urlPart)) {
      urls.push(request.url());
    }
  });
  return urls;
}

/** CDP-задержка сети: полёт мутации удлиняется — disable-кнопка в полёте
 * наблюдаема (прецедент double-submit-guard). */
async function holdLatency(page: Page, latencyMs: number): Promise<() => Promise<void>> {
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Network.enable');
  await cdp.send('Network.emulateNetworkConditions', {
    offline: false,
    latency: latencyMs,
    downloadThroughput: -1,
    uploadThroughput: -1,
  });
  return async () => {
    await cdp.send('Network.emulateNetworkConditions', {
      offline: false,
      latency: 0,
      downloadThroughput: -1,
      uploadThroughput: -1,
    });
  };
}

/** Два клика одной кнопкой в одном JS-таске — окно до перерисовки (#1113). */
async function doubleClickInOneTask(button: Locator): Promise<void> {
  await button.evaluate((element) => {
    const el = element as HTMLElement;
    el.click();
    el.click();
  });
}

test.describe('удаление правила задачи — объектный срез', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('невыполненные сносятся, журнал выполненных остаётся; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);

    // Два одноразовых правила на сегодня: одно останется невыполненным
    // (сносится удалением), второе выполняем API-вызовом — его строка
    // должна пережить удаление правила в журнале «Выполненные». Оба гасим
    // в finally — упавший ассерт не оставляет мусора следующим спекам
    // (канон tasks-feed-empty-filter).
    const uncompleted = await createRule(page, {
      title: 'E2E удалить правило',
      repeat: 'once',
      dueDate: todayIso(),
    });
    const completedRule = await createRule(page, {
      title: 'E2E журнал выполненных',
      repeat: 'once',
      dueDate: todayIso(),
    });
    try {
      await runDeleteFlow(page, testInfo, { uncompleted, completedRule });
    } finally {
      await page.request.delete(`/api/properties/${APARTMENT}/tasks/rules/${uncompleted}`);
      await page.request.delete(`/api/properties/${APARTMENT}/tasks/rules/${completedRule}`);
      // Журнал переживает правила (семантика фичи) — выполненные строки
      // гасим отдельным контрактом: на постоянном стеке (live-up) иначе
      // накапливаются одноимённые строки журнала.
      await page.request.delete(`/api/properties/${APARTMENT}/tasks/completed`);
    }
  });

  /** Тело сценария вынесено из try: cleanup в finally не должен глотать
   * ассерты. */
  async function runDeleteFlow(
    page: Page,
    testInfo: TestInfo,
    rules: { readonly uncompleted: string; readonly completedRule: string },
  ): Promise<void> {
    const { uncompleted, completedRule: journalRule } = rules;
    const active = await page.request.get(
      `/api/properties/${APARTMENT}/tasks?completed=false&limit=500`,
    );
    expect(active.ok(), 'чтение активных задач').toBe(true);
    const items = (await active.json()) as {
      items: ReadonlyArray<{ id: string; ruleId: string }>;
    };
    const toComplete = items.items.find((task) => task.ruleId === journalRule);
    if (toComplete === undefined) {
      throw new Error('вхождение правила журнала материализовалось');
    }
    const done = await page.request.post(
      `/api/properties/${APARTMENT}/tasks/${toComplete.id}/complete`,
    );
    expect(done.ok(), 'выполнение задачи').toBe(true);

    // Список: невыполненная строка на месте, журнал с выполненной.
    // Секция раскрывается кликом по заголовку; клик может уйти в замену
    // узла при перечитывании — добиваемся раскрытия повторами (канон
    // устойчивых спек: ассерт внутри toPass).
    await page.goto(`/properties/${APARTMENT}/tasks`);
    await expect(page.getByText('E2E удалить правило')).toBeVisible();
    const completedSection = page.getByTestId('section-completed');
    await expect(completedSection).toBeVisible();
    const trigger = completedSection.getByRole('button', { name: /Выполненные/ });
    const journalRow = page.getByText('E2E журнал выполненных');
    await expect(async () => {
      if (!(await journalRow.isVisible())) {
        await trigger.click();
      }
      await expect(journalRow).toBeVisible();
    }).toPass();

    // Удаление невыполненного правила: корзина в хедере правки, без
    // подтверждения; тост — уже на списке (goBack по истории).
    await openEditFromList(page, 'E2E удалить правило', uncompleted);
    await page.getByTestId('task-edit-delete').click();
    await expect(page.getByText('Задача удалена')).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/properties/${APARTMENT}/tasks$`));

    // Невыполненное снесено, журнал не тронут: строка пока ссылается
    // на своё живое правило.
    await expect(page.getByText('E2E удалить правило')).toHaveCount(0);
    await expect(page.getByTestId('section-completed')).toBeVisible();
    const journal = await page.request.get(
      `/api/properties/${APARTMENT}/tasks?completed=true&limit=500`,
    );
    expect(journal.ok(), 'чтение журнала').toBe(true);
    const journalItems = (await journal.json()) as {
      items: ReadonlyArray<{ id: string; ruleId: string | null }>;
    };
    const journaled = journalItems.items.find((task) => toComplete.id === task.id);
    expect(journaled, 'строка журнала на месте').toBeDefined();
    expect(journaled?.ruleId, 'правило журнала ещё живо').toBe(journalRule);

    await captureScreen(page, testInfo, 'task-delete-list-with-journal-mobile');

    // Удаление правила выполненного: у журнальной строки UI-пути в правку
    // нет (строки выполненных некликабельны) — вход прямым goto, то есть
    // свежим документом; goBack отсюда уходит настоящим переходом между
    // документами, и портал тоста умирает вместе с ним — тост здесь не
    // проверяем (в проде вход в правку всегда SPA-навигацией, тост покрыт
    // удалением выше). Контракт: правило снесено, строка журнала осталась
    // с обнулённым rule_id.
    await page.goto(`/properties/${APARTMENT}/tasks/${journalRule}/edit`);
    await page.getByTestId('task-edit-delete').click();
    // DELETE уходит из onSuccess после ответа сервера — контракт добиваем
    // опросом, а не сном (канон устойчивых спек).
    await expect(async () => {
      const read = await page.request.get(
        `/api/properties/${APARTMENT}/tasks/rules/${journalRule}`,
      );
      expect(read.status(), 'правило снесено на бэке').toBe(404);
    }).toPass();
    const reread = await page.request.get(
      `/api/properties/${APARTMENT}/tasks?completed=true&limit=500`,
    );
    expect(reread.ok(), 'перечтение журнала').toBe(true);
    const rereadItems = (await reread.json()) as {
      items: ReadonlyArray<{ id: string; ruleId: string | null }>;
    };
    const survived = rereadItems.items.find((task) => toComplete.id === task.id);
    expect(survived, 'журнал пережил удаление своего правила').toBeDefined();
    expect(survived?.ruleId, 'rule_id у строки журнала обнулён').toBeNull();
  }

  test('полный доступ удаляет (D9 #1309): корзина на месте, правило сносится', async ({
    page,
  }) => {
    await openCabinetWithSessionToken(page, seededMemberSessionToken());

    // Одноразовое правило без срока от сессии участника (сид не мутируем):
    // материализуется сразу в «Без срока».
    const ruleId = await createRule(page, {
      title: 'E2E правило участника',
      repeat: 'once',
    });

    try {
      await openEditFromList(page, 'E2E правило участника', ruleId);
      await expect(page.getByTestId('task-edit-delete')).toBeVisible();
      await page.getByTestId('task-edit-delete').click();

      await expect(page.getByText('Задача удалена')).toBeVisible();
      await expect(page).toHaveURL(new RegExp(`/properties/${APARTMENT}/tasks$`));
      await expect(page.getByText('E2E правило участника')).toHaveCount(0);
    } finally {
      // Упавший ассерт до удаления не оставляет правило следующим спекам.
      await page.request.delete(`/api/properties/${APARTMENT}/tasks/rules/${ruleId}`);
    }
  });

  test('двойной клик по корзине одним таском — ровно один DELETE (#1113)', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    const ruleId = await createRule(page, {
      title: 'E2E гард удаления',
      repeat: 'once',
      dueDate: todayIso(),
    });
    const deletes = trackDeletes(page, '/tasks/rules/');

    try {
      await openEditFromList(page, 'E2E гард удаления', ruleId);

      // Мутация в полёте: кнопка глохнет (disabled), второй клик того же
      // таска до перерисовки гардом useGuardedMutation дропается.
      const release = await holdLatency(page, 600);
      const trash = page.getByTestId('task-edit-delete');
      await doubleClickInOneTask(trash);
      await expect(trash).toBeDisabled();
      await release();

      expect(deletes).toHaveLength(1);
      await expect(page.getByText('Задача удалена')).toBeVisible();
    } finally {
      await page.request.delete(`/api/properties/${APARTMENT}/tasks/rules/${ruleId}`);
    }
  });
});

test.describe('удаление правила задачи — без объекта', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('плоский срез: из ленты в правку, удаление возвращает в ленту', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);

    // Правило без срока материализуется сразу в «Без срока» (канон
    // sorting-persistence) — строка ленты открывает плоскую правку (#523).
    const ruleId = await createRule(
      page,
      { title: 'E2E общая задача', repeat: 'once' },
      '/api/tasks/rules',
    );

    try {
      await page.goto('/tasks');
      await expect(page.getByText('E2E общая задача')).toBeVisible();
      await page.getByText('E2E общая задача').click();
      await expect(page).toHaveURL(new RegExp(`/tasks/${ruleId}/edit$`));

      await page.getByTestId('task-edit-delete').click();
      await expect(page.getByText('Задача удалена')).toBeVisible();

      // Возврат в ленту по истории (вход честный), строка исчезла.
      await expect(page).toHaveURL(/\/tasks$/);
      await expect(page.getByText('E2E общая задача')).toHaveCount(0);
    } finally {
      await page.request.delete(`/api/tasks/rules/${ruleId}`);
    }

    const read = await page.request.get(`/api/tasks/rules/${ruleId}`);
    expect(read.status(), 'правило снесено на бэке').toBe(404);
  });
});

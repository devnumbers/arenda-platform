import {
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';
import type { Page } from '@playwright/test';

// Гард сдвига страницы при Radix-оверлеях (#925): на Windows Chrome скроллбары
// классические (занимают место в раскладке), на macOS — оверлейные, поэтому
// локально баг не виден, а headless Chromium не отдаёт место под скроллбар
// ни при каких флагах (проверено зондом: --hide-scrollbars, channel chromium,
// disable-features OverlayScrollbar — гэп всегда 0). Поэтому стенд строится
// на каскаде, а не на рендере скроллбара:
//
//   1. Production-CSS резервирует гюттер (scrollbar-gutter: stable на html,
//      #865) — на Windows это 15–17px всегда, даже под лоченым body. Стенд
//      имитирует его через html { padding-right: 15px } — layout-геометрия
//      та же (контентный бокс html уже на ширину скроллбара).
//   2. react-remove-scroll-bar при локе (исходник 2.3.8, getStyles) вешает на
//      body[data-scroll-locked] { margin-right: <gap>px !important }, не зная
//      про гюттер, — на Windows это двойная компенсация и прыжок колонки.
//      Стенд инжектит копию этого правила с Windows-гэпом 15px и держит её
//      последним стилем в head, пока лок активен (на реальной странице
//      singleton либы аппендится в момент монтирования оверлея — так же
//      позже нашего CSS; для gap=0 в headless конкуренции нет, поэтому
//      копия ре-аппендится по MutationObserver на data-scroll-locked).
//
// Фикс в globals.css — html body[data-scroll-locked] { margin-right: 0
// !important }: специфичность (0,1,2) бьёт и либу, и копию независимо от
// порядка стилей. Инвариант теста: контент не сдвигается, лок работает.
const STAND_IN_CSS = `
  html { padding-right: 15px; }
  body[data-scroll-locked] { margin-right: 15px !important; }
`;

interface LockState {
  locked: boolean;
  overflow: string;
  marginRight: string;
  gutter: string;
}

/** Копия стиля react-remove-scroll-bar: живёт со старта страницы (stand-in
 * гюттера обязан действовать и до лока) и при каждом локе передвигается
 * последним элементом head — singleton либы аппендится при монтировании
 * оверлея и должен остаться РАНЬШЕ копии, как на Windows, где его
 * margin-right выигрывает каскад у production-CSS до фикса. */
async function standInWindowsScrollbar(page: Page): Promise<void> {
  await page.addInitScript((css) => {
    document.addEventListener('DOMContentLoaded', () => {
      const style = document.createElement('style');
      style.id = 'scroll-shift-stand-in';
      style.textContent = css;
      document.head.appendChild(style);
      new MutationObserver(() => {
        if (document.body.hasAttribute('data-scroll-locked')) {
          requestAnimationFrame(() => document.head.appendChild(style));
        }
      }).observe(document.body, { attributes: true, attributeFilter: ['data-scroll-locked'] });
    });
  }, STAND_IN_CSS);
}

async function lockState(page: Page): Promise<LockState> {
  return page.evaluate(() => ({
    locked: document.body.hasAttribute('data-scroll-locked'),
    overflow: getComputedStyle(document.body).overflow,
    marginRight: getComputedStyle(document.body).marginRight,
    gutter: getComputedStyle(document.documentElement).scrollbarGutter,
  }));
}

/** X левого края первого h1 — ин-флоу контент колонки, центр которой и прыгает. */
async function contentX(page: Page): Promise<number> {
  const x = await page.evaluate(() => {
    const el = document.querySelector('h1');
    return el ? el.getBoundingClientRect().x : Number.NaN;
  });
  expect(x, 'на странице есть h1 — проба сдвига').not.toBeNaN();
  return x;
}

/** Общий хвост обоих тестов: лок активен, компенсация обнулена, контент
 * на месте. */
async function expectLockedWithoutShift(page: Page, xBefore: number): Promise<void> {
  const state = await lockState(page);
  expect(state.locked, 'лок скролла активен').toBe(true);
  expect(state.overflow).toBe('hidden');
  expect(state.marginRight, 'компенсация react-remove-scroll-bar обнулена').toBe('0px');

  const xDuring = await contentX(page);
  expect(Math.abs(xDuring - xBefore), 'контент не сдвинулся').toBeLessThan(0.5);
}

test.describe('страница не сдвигается при Radix-оверлеях (#925)', () => {
  // Низкий десктопный вьюпорт — страница гарантированно скроллится,
  // как на реальном Windows-ноутбуке.
  test.use({ viewport: { width: 1280, height: 620 } });

  test.beforeEach(async ({ page, seededUser }) => {
    await standInWindowsScrollbar(page);
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/properties');
    await expect(page.getByRole('heading', { name: 'Объекты', exact: true })).toBeVisible();

    // Пейринг с #865 на месте: гюттер зарезервирован production-CSS.
    expect((await lockState(page)).gutter).toBe('stable');
  });

  test('Radix Dialog («Поддержка») не сдвигает контент', async ({ page }) => {
    const xBefore = await contentX(page);

    await page.getByRole('button', { name: 'Поддержка' }).click();
    const dialog = page.getByRole('dialog', { name: 'Связаться с нами' });
    await expect(dialog).toBeVisible();
    await expectLockedWithoutShift(page, xBefore);

    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();

    // Radix снимает лок после exit-анимации — ждём снятия, а не читаем сразу.
    // clip visible — канон body из globals.css (overflow-x: clip).
    await expect
      .poll(() => lockState(page), 'лок снят после закрытия')
      .toMatchObject({ locked: false, overflow: 'clip visible' });
  });

  test('Radix DropdownMenu (кебаб объекта) не сдвигает контент', async ({ page }) => {
    await page.getByRole('link', { name: 'Квартира на Ленина' }).click();
    // Ждём контент страницы (не скелетон): замер до конца гидрации ловит
    // смену RSC-дерева, а не сдвиг скроллбара (канон #698).
    await expect(page.getByTestId('property-manage-list')).toBeVisible();

    const xBefore = await contentX(page);

    await page.getByRole('button', { name: 'Действия с объектом' }).click();
    const menu = page.getByRole('menu');
    await expect(menu).toBeVisible();
    // modal=true у DropdownMenu тоже лочит скролл через react-remove-scroll.
    await expectLockedWithoutShift(page, xBefore);

    await page.keyboard.press('Escape');
    await expect(menu).toBeHidden();
    await expect
      .poll(() => lockState(page), 'лок снят после закрытия')
      .toMatchObject({ locked: false });
  });
});

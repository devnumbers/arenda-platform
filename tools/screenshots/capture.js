const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

const BASE = 'http://localhost:3000/properties/new';
const LOGIN = 'http://localhost:3000/login';
const PHONE = '+7 (915) 038-06-63';
const PHONE_RAW = '+79150380663';
const OUTPUT = path.resolve(__dirname, '../../screenshots');

fs.mkdirSync(OUTPUT, { recursive: true });

const contexts = [
  { name: 'mobile', width: 375, height: 812 },
  { name: 'desktop', width: 1280, height: 900 },
];

const steps = [
  { n: 1, draft: { step: 1 } },
  { n: 2, draft: { step: 2, type: 'apartment' } },
  { n: 3, draft: { step: 3, type: 'apartment', address: 'г Москва, ул Лобановский Лес, д 12', name: 'Квартира на Лобановском', description: 'Уютная квартира' } },
  { n: 4, draft: { step: 4 } },
];

function extractLatestCode(logPath) {
  const log = fs.readFileSync(logPath, 'utf8');
  const matches = [...log.matchAll(/Код подтверждения: (\d{6})/g)];
  return matches.length > 0 ? matches[matches.length - 1][1] : null;
}

async function login(page) {
  await page.goto(LOGIN, { waitUntil: 'networkidle' });
  await page.getByLabel('Телефон').fill(PHONE);
  await page.getByRole('button', { name: 'Войти' }).click();
  await page.waitForSelector('text=Введите код', { timeout: 10000 });

  const code = extractLatestCode('/tmp/backend.log');
  if (!code) throw new Error('Could not extract login code from backend log');

  await page.getByLabel('6-значный код').fill(code);
  await page.waitForURL('http://localhost:3000/', { timeout: 10000 });
}

(async () => {
  const browser = await chromium.launch();

  for (const ctx of contexts) {
    const context = await browser.newContext({
      viewport: { width: ctx.width, height: ctx.height },
    });
    const page = await context.newPage();
    await login(page);

    for (const step of steps) {
      await page.goto(BASE, { waitUntil: 'networkidle' });
      await page.evaluate((data) => {
        sessionStorage.setItem('property-create-draft', JSON.stringify(data));
      }, step.draft);
      await page.reload({ waitUntil: 'networkidle' });
      await page.waitForTimeout(500);
      await page.screenshot({
        path: path.join(OUTPUT, `step${step.n}-${ctx.name}.png`),
        fullPage: false,
      });
    }

    await context.close();
  }

  await browser.close();
  console.log('Screenshots saved to', OUTPUT);
})();

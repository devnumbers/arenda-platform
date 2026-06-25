const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

const BASE = 'http://localhost:3000/leases/new';
const LOGIN = 'http://localhost:3000/login';
const OUTPUT = path.resolve(__dirname, '../../screenshots');

fs.mkdirSync(OUTPUT, { recursive: true });

const contexts = [
  { name: 'mobile', width: 375, height: 812 },
  { name: 'desktop', width: 1280, height: 900 },
];

function makePhone() {
  const suffix = Math.floor(1000 + Math.random() * 9000);
  return `+7 (915) 038-${String(suffix).slice(0, 2)}-${String(suffix).slice(2)}`;
}

function extractLatestCode(logPath) {
  const log = fs.readFileSync(logPath, 'utf8');
  const matches = [...log.matchAll(/Код подтверждения: (\d{6})/g)];
  return matches.length > 0 ? matches[matches.length - 1][1] : null;
}

async function login(page) {
  const phone = makePhone();

  await page.goto(LOGIN, { waitUntil: 'networkidle' });
  await page.evaluate(() => {
    window.localStorage.removeItem('arenda:lastSmsSendAt');
  });
  await page.reload({ waitUntil: 'networkidle' });
  await page.getByLabel('Телефон').fill(phone);
  await page.waitForSelector('button[type="submit"]:not(:disabled)', { timeout: 10000 });
  await page.getByRole('button', { name: 'Войти' }).click();
  await page.waitForSelector('text=Введите код', { timeout: 10000 });
  await page.waitForTimeout(1000);

  const code = extractLatestCode('/tmp/backend.log');
  if (!code) throw new Error('Could not extract SMS code from backend log');

  await page.getByLabel('6-значный код').fill(code);
  await page.waitForURL('http://localhost:3000/', { timeout: 10000 });
}

async function createFreeProperty(page) {
  const response = await page.request.post('http://localhost:3000/api/properties', {
    data: {
      name: 'Квартира для скриншотов',
      type: 'apartment',
      address: 'г Москва, ул Лобановский Лес, д 12',
      description: 'Тестовый объект для создания аренды',
    },
  });

  if (!response.ok()) {
    const body = await response.text();
    throw new Error(`Failed to create property: ${response.status()} ${body}`);
  }

  const property = await response.json();
  if (!property.id) {
    throw new Error('Property response missing id');
  }
  return property.id;
}

const steps = [
  {
    name: 'step1',
    draft: { step: 1 },
  },
  {
    name: 'step2',
    draft: {
      step: 2,
      rentAmount: '50000',
      depositAmount: '10000',
      paymentDay: 5,
      startDate: '2026-07-01',
    },
  },
  {
    name: 'success',
    draft: { step: 3 },
  },
];

(async () => {
  const browser = await chromium.launch();

  for (const ctx of contexts) {
    const context = await browser.newContext({
      viewport: { width: ctx.width, height: ctx.height },
    });
    const page = await context.newPage();
    await login(page);
    const propertyId = await createFreeProperty(page);

    for (const step of steps) {
      await page.goto(`${BASE}?propertyId=${propertyId}`, { waitUntil: 'networkidle' });
      await page.evaluate((data) => {
        sessionStorage.setItem('lease-create-draft', JSON.stringify(data));
      }, step.draft);
      await page.reload({ waitUntil: 'networkidle' });
      await page.waitForTimeout(1500);
      await page.screenshot({
        path: path.join(OUTPUT, `lease-${step.name}-${ctx.name}.png`),
        fullPage: false,
      });
    }

    await context.close();
  }

  await browser.close();
  console.log('Screenshots saved to', OUTPUT);
})();

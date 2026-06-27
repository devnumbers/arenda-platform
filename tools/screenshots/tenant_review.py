from playwright.sync_api import sync_playwright
import os

ROOT = "/Users/smirnowwwivan/Nambers/arenda-planform"
OUT = os.path.join(ROOT, "screenshots")
os.makedirs(OUT, exist_ok=True)

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    context = browser.new_context(viewport={"width": 1440, "height": 900})
    context.add_cookies([
        {
            "name": "session_id",
            "value": "review-session",
            "domain": "localhost",
            "path": "/",
            "httpOnly": False,
            "secure": False,
            "sameSite": "Lax",
        }
    ])

    page = context.new_page()

    # Mock the tenant contact creation API so the success screen can be captured
    # without requiring a real authenticated backend session.
    page.route("**/api/tenant-contacts", lambda route, request: route.fulfill(
        status=201,
        content_type="application/json",
        body='{"id":"00000000-0000-0000-0000-000000000000","name":"Иван","surname":"Иванов","patronymic":null,"phone":"+79001234567","email":null,"comment":null,"created_at":"2026-06-25T00:00:00Z","updated_at":"2026-06-25T00:00:00Z"}'
    ))

    page.goto("http://localhost:3000/tenants/new", wait_until="networkidle")

    # Desktop form
    page.set_viewport_size({"width": 1440, "height": 900})
    page.screenshot(path=os.path.join(OUT, "tenant-review-desktop.png"), full_page=True)

    # Mobile form
    page.set_viewport_size({"width": 375, "height": 812})
    page.screenshot(path=os.path.join(OUT, "tenant-review-mobile.png"), full_page=True)

    # Fill form and submit to reach the success screen.
    page.set_viewport_size({"width": 1440, "height": 900})
    page.locator("label:has-text('Имя') + * input, label:has-text('Имя') input").first.fill("Иван")
    page.locator("label:has-text('Фамилия') + * input, label:has-text('Фамилия') input").first.fill("Иванов")
    page.locator("input[placeholder*='+7']").first.fill("+79001234567")
    page.locator("button:has-text('Добавить арендатора')").click()

    page.wait_for_selector("text=Арендатор добавлен", timeout=5000)
    page.screenshot(path=os.path.join(OUT, "tenant-review-success-desktop.png"), full_page=True)

    page.set_viewport_size({"width": 375, "height": 812})
    page.screenshot(path=os.path.join(OUT, "tenant-review-success-mobile.png"), full_page=True)

    browser.close()

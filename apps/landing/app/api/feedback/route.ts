// Прокси формы «Задать вопрос» (карта #1010): браузер стучится в свой же
// origin, роут пересылает вопрос серверным POST в бекенд по BACKEND_URL —
// единственный канал лендинга к бекенду (ADR 0063), почта и SMTP-секреты
// живут там. Honeypot «website» (невидимое поле для людей) заполнен —
// отвечаем успехом, не беспокоя бекенд и почтовый ящик получателя.
const BACKEND_TIMEOUT_MS = 5000;

const FEEDBACK_EMAIL_MAX = 254;
const FEEDBACK_MESSAGE_MAX = 1000;

export async function POST(request: Request) {
  let body: { email?: unknown; message?: unknown; website?: unknown };
  try {
    body = await request.json();
  } catch {
    return Response.json({ error: "Некорректный запрос" }, { status: 400 });
  }

  if (typeof body.website === "string" && body.website.trim() !== "") {
    return new Response(null, { status: 204 });
  }

  const email = typeof body.email === "string" ? body.email.trim() : "";
  const message = typeof body.message === "string" ? body.message.trim() : "";
  if (
    email === "" ||
    message === "" ||
    email.length > FEEDBACK_EMAIL_MAX ||
    message.length > FEEDBACK_MESSAGE_MAX
  ) {
    return Response.json({ error: "Заполните оба поля" }, { status: 400 });
  }

  const backend = process.env.BACKEND_URL ?? "http://localhost:8080";
  try {
    const res = await fetch(`${backend}/feedback`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, message }),
      signal: AbortSignal.timeout(BACKEND_TIMEOUT_MS),
      cache: "no-store",
    });
    if (res.status === 204) {
      return new Response(null, { status: 204 });
    }
    return Response.json({ error: "Не удалось отправить сообщение" }, { status: res.status });
  } catch {
    return Response.json({ error: "Не удалось отправить сообщение" }, { status: 502 });
  }
}

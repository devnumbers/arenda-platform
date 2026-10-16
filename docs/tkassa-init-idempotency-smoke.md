# Протокол стейдж-smoke: идемпотентность Init по OrderId

Фиксированная ручная проверка контракта Т-Кассы (спека #419, тикет #423).
Отвечает на один вопрос: **что делает `Init` при повторном вызове с тем же
`OrderId` — возвращает существующий платёж или создаёт новый?**

## Зачем

Официальная документация требует уникальности `OrderId`, но не описывает
поведение при повторе (находка F10 ревью
`docs/research/2026-08-23-billing-review-tkassa-integration.md`). На
предположении «повторный Init по тому же OrderId возвращает существующий
платёж» держатся два механизма адаптера
(`apps/backend/internal/billing/adapters/payment/tkassa/retry_transport.go`):

1. ретрай Init после таймаута, когда запрос уже ушёл на провода
   (`retryOnTimeoutMethods`);
2. флаг `T_KASSA_RETRY_MUTATIONS` — ретраи HTTP 5xx для мутаций Init/Charge
   (по умолчанию выключен именно до подтверждения этой проверкой).

Если повторный Init создаёт новый платёж, ретрай мутации мог бы задвоить
платёжную сессию (Init) — поэтому проверка обязательна до включения флага.
Если повторный Init отвечает ошибкой, это дополнительно ломает recovery-путь
повторного Init (`application/subscription_service.go`) — ему потребуется
новый OrderId на каждую попытку.

## Когда запускать

- один раз на стейдж-терминале до первого включения `T_KASSA_RETRY_MUTATIONS`;
- повторно после каждого re-vendor спеки Т-Кассы (ADR 0016) или смены
  терминала.

## Предусловия

- Стейдж-окружение с `PAYMENT_PROVIDER=tkassa`, боевой терминал без префикса
  `DEMO`, base URL `https://rest-api-test.tinkoff.ru/v2` (whitelist IP —
  ADR 0017 «Тестовая среда и аттестация»).
- `T_KASSA_TERMINAL_KEY` и `T_KASSA_PASSWORD` стейдж-терминала.
- `curl`, `openssl`, `uuidgen`.

## Шаги

1. Зафиксировать `ORDER_ID` (свежий UUID) и собрать минимальное тело Init:
   разовый платёж без сохранения реквизитов — `PayType="O"`,
   `DATA.OperationInitiatorType="0"`, без `Recurrent`/`RedirectDueDate`.
2. Посчитать `Token` по алгоритму ADR 0010/0017: из подписи исключаются
   `Token` и вложенный объект `DATA`, добавляется `Password`, значения
   конкатенируются в порядке сортировки ключей, SHA-256 hex-lowercase.
3. Отправить Init, зафиксировать `Success` и `PaymentId`.
4. **Не меняя ни одного поля**, отправить то же тело повторно (токен тот же —
   тело идентично).
5. Сравнить `PaymentId` ответов; для контроля сверить состояние через
   `GetState` по `PaymentId`.
6. Негативный контроль: повторить шаги 1–3 с новым `ORDER_ID` — должен
   вернуться другой `PaymentId` (проверка не вакуумна).

Готовый скрипт (значения для конкатенации уже отсортированы по ключам:
`Amount`, `OrderId`, `Password`, `PayType`, `TerminalKey`):

```bash
set -euo pipefail
TERMINAL_KEY=...   # стейдж-терминал
PASSWORD=...       # пароль терминала
ORDER_ID="$(uuidgen | tr 'A-Z' 'a-z')"
TOKEN="$(printf '10000%s%sO%s' "$ORDER_ID" "$PASSWORD" "$TERMINAL_KEY" \
  | openssl dgst -sha256 -hex | awk '{print $2}')"

request() {
  curl -sS -X POST https://rest-api-test.tinkoff.ru/v2/Init \
    -H 'Content-Type: application/json' \
    -d "{\"TerminalKey\":\"$TERMINAL_KEY\",\"Amount\":10000,\"OrderId\":\"$ORDER_ID\",\
\"PayType\":\"O\",\"DATA\":{\"OperationInitiatorType\":\"0\"},\"Token\":\"$TOKEN\"}"
}

echo "ORDER_ID=$ORDER_ID"
request | tee /tmp/init-1.json   # зафиксировать PaymentId
request | tee /tmp/init-2.json   # тот же OrderId, то же тело
```

Сверка `GetState` (подпись — по ключам `Password`, `PaymentId`, `TerminalKey`):

```bash
PAYMENT_ID="$(jq -r .PaymentId /tmp/init-1.json)"
STATE_TOKEN="$(printf '%s%s%s' "$PASSWORD" "$PAYMENT_ID" "$TERMINAL_KEY" \
  | openssl dgst -sha256 -hex | awk '{print $2}')"
curl -sS -X POST https://rest-api-test.tinkoff.ru/v2/GetState \
  -H 'Content-Type: application/json' \
  -d "{\"TerminalKey\":\"$TERMINAL_KEY\",\"PaymentId\":\"$PAYMENT_ID\",\"Token\":\"$STATE_TOKEN\"}"
```

## Критерии исхода

| Исход | Признак | Решение |
|---|---|---|
| Идемпотентен | оба ответа `Success=true`, `PaymentId` совпадают | включить `T_KASSA_RETRY_MUTATIONS=true` на stage, понаблюдать метрики `payment.provider` несколько дней, затем включить в prod; исход записать в таблицу ниже и в ADR 0016 как проверенный факт. Важно: флаг покрывает и Charge — smoke доказывает безопасность только Init; повторный Charge не идемпотентен по построению (тело без OrderId), так что решение «включать с Charge или держать флаг выкл» принимает дежурный, понимая остаточный риск двойного списания при 5xx после обработки запроса шлюзом |
| Создаёт новый платёж | второй ответ `Success=true` с новым `PaymentId` | флаг не включать; убрать `Init` из `retryOnTimeoutMethods` (то же предположение); завести тикет на re-Init с новым OrderId в recovery-пути |
| Отвечает ошибкой | второй ответ `Success=false` | флаг не включать; убрать `Init` из `retryOnTimeoutMethods`; recovery-путь повторного Init требует нового OrderId — тикет |

## Журнал прогонов

| Дата | Терминал | Base URL | Исход | PaymentId №1 / №2 | Решение |
|---|---|---|---|---|---|
| — | — | — | не запускался | — | флаг `T_KASSA_RETRY_MUTATIONS` выключен во всех окружениях |

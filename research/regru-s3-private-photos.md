# Reg.ru Object Storage (S3) и приватное хранение фото пользователей

Дата: 2026-10-07.
Контекст: Arenda Platform — Go-бэкенд (aws-sdk-go-v2) + Next.js-фронтенд. Фото (аватары, фото объектов) — персональные данные, публичный доступ к ним запрещён по требованиям продукта и 152-ФЗ.

> Документ — исследовательский, не меняет поведение системы. Все факты о Reg.ru взяты из официальной документации Рег.облака (reg.cloud / help.reg.ru), ссылки на источники — в конце. Пункты, которые официально не задокументированы и требуют проверки на живом аккаунте, помечены **[ПРОВЕРИТЬ]**.

---

## TL;DR — рекомендуемая схема

1. **Один приватный бакет** на окружение (dev/prod раздельно), тип доступа «По ключам» (это значение по умолчанию). Публичный доступ не включать никогда.
2. **Выдача фото — короткоживущие presigned GET** (5–15 минут), генерируемые бэкендом после авторизации и проверки владения/доступа. Это основной путь раздачи.
3. **Загрузка — presigned PUT напрямую из браузера** с подписанным `Content-Type` (только `image/jpeg`/`image/png`/`image/webp`), с фиксированным сервером ключом (UUID, без PII). Небольшие фото можно лить и через бэкенд.
4. **Валидация после загрузки обязательна**: `http.DetectContentType` по магическим байтам, лимит размера (`HeadObject`), декодирование и **ре-энкод** превью (заодно снимает EXIF, включая GPS-координаты). SVG не принимать вообще.
5. **Проксирование через бэкенд оставить для превью** (мелкие файлы, рендерятся на лету и кэшируются) — либо наоборот, всё через presigned GET, если хочется снять нагрузку с бэкенда. Решение зависит от требований к кэшированию (см. раздел 2.1).
6. Учитывать лимиты Reg.ru: **1000 чтений/с и 500 записей/с на бакет** (10000/5000 на пользователя) — при большом трафике presigned GET (нагрузка идёт мимо бэкенда и суммируется по бакету) выгоднее проксирования.

---

## 1. Reg.ru Object Storage — что это и что умеет

### 1.1 Продукт и S3-совместимость

- Продукт: «Объектное хранилище S3» от Рег.ру (Рег.облако), позиционируется как аналог Amazon S3 для российского бизнеса. Управление — через S3 API и Terraform; заявлена «обновление и поддержка S3-совместимого API» (матрица ответственности на странице продукта).
- Доступ — «через личный кабинет либо через любые S3-совместимые инструменты»: официальные инструкции построены вокруг **AWS CLI (v1 и v2)**, `s3api`-команд и rclone (провайдер «Any other S3 compatible provider»). Значит, любой AWS SDK, включая aws-sdk-go-v2, должен работать через endpoint + ключи.
- Подпись запросов: в официальной инструкции по rclone регион оставляется пустым, дефолт — «Will use **v4 signatures** and an empty region». То есть SigV4 — штатный путь (aws-sdk-go-v2 умеет только SigV4, так что это ок). Поддержка SigV2 не документирована — считать, что её нет.

### 1.2 Endpoint, регион и форматы URL

- **Endpoint**: `https://s3.regru.cloud` — упоминается в инструкциях (CORS, rclone, presign). Точный S3 API Endpoint выдаётся в панели: «Мои ресурсы → Хранилище S3 → Ключи доступа → Параметры набора ключей → строка **S3 API Endpoint**».
- **Регион**: инструкции явно говорят оставить поле региона пустым (и в AWS CLI, и в rclone). Для aws-sdk-go-v2 это значит, что регион можно задать фиктивно (например, `us-east-1` — на подписание не влияет, если endpoint переопределён) **[ПРОВЕРИТЬ]**.
- **Формат адресации — path-style**: официальная статья показывает «Path-style ссылка» вида
  `https://s3.regru.cloud/<bucket_name>/<object_id>`.
  Следствие для SDK: в aws-sdk-go-v2 нужно выставить `s3.Options.UsePathStyle = true` (по умолчанию SDK использует virtual-hosted стиль `bucket.s3.regru.cloud`, который Reg.ru не документирует). В AWS CLI v2 — настройка `addressing_style = path` в `.aws/config` **[ПРОВЕРИТЬ — CLI-инструкции Reg.ru работают и без неё, но у CLI для custom-endpoint с точками есть свои правила вывода стиля]**.
- **Виртуальные бакеты по своему домену**: возможны только через статический website-хостинг и публичный бакет, и — критично — **TLS для собственных доменов не поддерживается, доступ только по plain HTTP**. Для приватных фото собственный домен не подходит вообще; используем `s3.regru.cloud` (HTTPS) или отдаём через свой бэкенд.

### 1.3 Бакеты, ключи доступа

- Бакеты создаются в панели («Новый ресурс → Бакет хранилища S3»): имя, максимальный размер бакета (по желанию), тип доступа. Квота хранилища по умолчанию — **10 ГБ**, меняется в панели. Количество бакетов не ограничено (страница продукта).
- **Ключи доступа** — «наборы ключей»: имя набора + S3 API Endpoint + Access Key ID + Secret Access Key; пару можно перегенерировать, набор — удалить. Сервисных IAM-полик/пользователей уровня AWS нет — права раздаются привязкой правил политик к наборам ключей (см. 1.4). Рекомендация: отдельный набор ключей на каждое окружение/сервис, ротация через «Сгенерировать новую пару».

### 1.4 Приватность по умолчанию и модель ACL/политик

- **По умолчанию бакет приватный**: при создании выбирается тип доступа, из двух значений приватный — «**По ключам**» (нужна аутентификация Access Key ID / Secret Key); публичный вариант называется «Открыт для всех». То есть «unsafe by default» ситуации, как в старых AWS-бакетах, здесь нет — но это надо подтверждать при каждом создании бакета.
- Модель разграничения — **политики доступа** (аналог bucket policy, без IAM):
  - тип доступа «По ключам» / «Открыт для всех» / «Пользовательский» (последний выставляется автоматически, если в политике есть собственные правила);
  - правила: набор ключей, действия (Чтение / Чтение и запись / Все действия / пользовательский набор), **область действия** — бакет (`bucket`), объект (`bucket/some/key`), префикс (`bucket/some/path/*`), все объекты (`bucket/*`);
  - **условия** (condition keys) — есть список ключей и операторов, какие именно ключи доступны (например, IP-условия), документация на списке не конкретизирует — **[ПРОВЕРИТЬ]**;
  - семантика — **default deny**: «Если ни одна из политик явно не разрешает действие, доступ по умолчанию считается запрещённым»;
  - **опасная операция**: «При изменении типа доступа все политики безвозвратно удалятся» — переключение типа доступа в панели сносит все правила.
- Классические S3 canned-ACL (`x-amz-acl`) в документации не описаны; в разделе «доступ без аутентификации» упомянуты вкладки «ACL» и «policy.json» как два способа открыть публичный доступ. Точная поддержка ACL-заголовков — **[ПРОВЕРИТЬ]**; для нашей схемы (приватный бакет + presigned) ACL вообще не нужны.
- Вывод для продукта: приватность держится на (а) типе доступа «По ключам», (б) отсутствии публичных правил в политиках, (в) presigned URL для точечной выдачи.

### 1.5 Presigned URL (GET / PUT, срок жизни)

- Официально задокументированы **presigned GET**: `aws s3 presign s3://bucket/object` — TTL по умолчанию **3600 секунд**, переопределяется `--expires-in n-seconds`. В FAQ страницы продукта «защита объектов с помощью подписанных URL» заявлена как штатная мера безопасности.
- **Presigned PUT** отдельной статьёй не описан, но это та же SigV4-presign-механика, что и в AWS; в CORS-статье Reg.ru явно разрешает методы `PUT` и `POST` для бакетов — то есть браузерные загрузки планируются как сценарий. Использовать presigned PUT можно, но **[ПРОВЕРИТЬ]** на живом аккаунте (генерация через aws-sdk-go-v2 `PresignPutObject` и загрузка из браузера).
- **Максимальный срок жизни** Reg.ru не документирует. Ориентир — ограничение SigV4 в самом AWS: максимум **7 дней (604 800 сек)** с долгосрочными ключами; мы всё равно будем выдавать 5–15 минут, так что лимит не критичен. Верхнюю границу на стороне Reg.ru при необходимости проверить вручную.
- Свойства presigned URL (из AWS-документации, применимо к любому SigV4-совместимому хранилищу): это **bearer-токены** — кто получил ссылку, тот имеет доступ до истечения; ссылка многоразовая до истечения; срок проверяется в момент HTTP-запроса (незавершённая загрузка не рвётся посреди файла).

### 1.6 CORS

- Поддерживается штатно: `aws s3api put-bucket-cors --bucket ... --endpoint-url https://s3.regru.cloud --cors-configuration file://cors-config.json`, проверка `get-bucket-cors`, удаление `delete-bucket-cors`. Есть и управление через панель (статья «Управление CORS policy в хранилище S3»).
- Поддерживаемые методы: **GET, PUT, POST, DELETE, OPTIONS, HEAD**; поля правил — `AllowedHeaders`, `AllowedMethods`, `AllowedOrigins` (конкретный домен вместо `*`), как в AWS.
- Пример из документации Reg.ru (для прода заменить `*` на конкретный origin!):

```json
{
    "CORSRules": [
        {
            "AllowedHeaders": ["*"],
            "AllowedMethods": ["GET", "PUT"],
            "AllowedOrigins": ["https://app.example.ru"]
        }
    ]
}
```

### 1.7 Шифрование на стороне хранилища

- **SSE-C** (шифрование ключами клиента, AES256) — поддерживается полностью: `PutObject`, `GetObject`, `HeadObject`, `CopyObject`, `CreateMultipartUpload`, `UploadPart`, `UploadPartCopy`; заголовки `x-amz-server-side-encryption-customer-*`; ключ хранится только как HMAC-SHA256, при потере ключа данные теряются.
- Базовое «шифрование данных» упомянуто на странице продукта в числе мер безопасности; какой именно режим по умолчанию (аналог SSE-S3) — не детализировано. Для наших задач: SSE-C опционально, по умолчанию достаточно серверного шифрования провайдера + запрет публичного доступа.
- In-transit: HTTPS на `s3.regru.cloud` (инструкция по AWS CLI использует `https://`). Есть отдельная статья про «Ошибка SSL-сертификата в AWS CLI» — для старых систем может понадобиться свежий CA-бандл.

### 1.8 Классы хранения и лимиты

- **Классы хранения: только standard**; «в будущем планируется добавление холодного класса» — на текущий момент IA/Glacier-аналогов нет, lifecycle-переходов между классами тоже нет.
- **Лимиты операций** (официальная статья):
  - на бакет: `max_read_ops = 1000`/с, `max_write_ops = 500`/с;
  - на пользователя: `max_read_ops = 10000`/с, `max_write_ops = 5000`/с;
  - на клиента: при полосе >10 Гбит/с возможно ограничение RPS и скорости.
- Лимиты на размер объекта/частей отдельно не документированы — считать действующими стандартные ограничения S3-совместимых хранилищ (5 ГБ на часть, 10 000 частей, 5 ТБ на объект) и **[ПРОВЕРИТЬ]** для больших файлов. Для фото (единицы–десятки МБ) неактуально.
- Ценообразование (страница продукта): плата за хранимый объём (в разных версиях страницы встречалось 2,19–2,63 ₽/ГБ/мес — фиксировать актуальную цену на момент закупки), без лимитов и оплаты исходящего трафика и операций по счётчику; возможность кастомного домена + SSL — из той же матрицы (но см. нюанс про TLS в 1.2).
- Прочее: тройная репликация, ЦОД Tier III в РФ (Москва/СПб/Самара), защита от DDoS L3/L4, соответствие 152-ФЗ (важно для персональных данных — фото арендаторов/объектов), инцидентоустойчивость исторически средняя (были аварии S3-кластера Рег.облака с восстановлением ~1 час — принимать во внимание для SLA).

### 1.9 Версионирование, lifecycle, multipart-очистка

- **Версионирование**: `put-bucket-versioning` (`Status=Enabled`), `list-object-versions`, `get-object --version-id`, `copy-object` c `?versionId=`, `delete-object --version-id` — всё поддерживается.
- **Lifecycle-конфигурация**: поддерживается `put-bucket-lifecycle-configuration`, в документации показано правило `NoncurrentVersionExpiration` (автоудаление неактуальных версий через N дней). Полный набор правил AWS (Expiration по префиксу, `AbortIncompleteMultipartUpload` и т.п.) не документирован — правило для абортов незавершённых multipart-загрузок **[ПРОВЕРИТЬ]**; пока чистить вручную.
- **Частично загруженные объекты**: чистятся через `s3api abort-multipart-upload` (или `DELETE /{bucket}/{key}?uploadId=...`) после `list-multipart-uploads` — то есть «сироты» multipart нужно убирать самостоятельно фоновым джобом.

### 1.10 Сводка нюансов по сравнению с AWS S3

| Аспект | AWS S3 | Reg.ru Object Storage |
|---|---|---|
| Приватность по умолчанию | бакет приватный | бакет приватный («По ключам») |
| IAM-пользователи/политики | IAM + bucket policy + ACL | наборы ключей + политики бакета с областями действия; default deny |
| Presigned URL | GET/PUT/HEAD, до 7 дней (SigV4) | GET задокументирован (дефолт 3600 с); PUT — через ту же SigV4-механику, требует живой проверки |
| CORS | полная поддержка | полная поддержка (GET/PUT/POST/DELETE/OPTIONS/HEAD) |
| Шифрование | SSE-S3/SSE-KMS/SSE-C | серверное шифрование заявлено; SSE-C (AES256) поддерживается; SSE-KMS нет |
| Классы хранения | Standard/IA/Glacier | только Standard; «холодный» анонсирован |
| Версионирование + lifecycle | да, все правила | версионирование да; lifecycle показан на `NoncurrentVersionExpiration`; остальные правила не документированы |
| Кастомный домен | CloudFront + TLS | только website-хостинг публичного бакета, **без TLS (HTTP)** |
| Лимиты операций | практически нет (по запросу) | 1000 r / 500 w ops/s на бакет; 10000/5000 на пользователя |
| Адресация | virtual-hosted по умолчанию | **path-style** (`https://s3.regru.cloud/bucket/key`) |
| Точка отказа / SLA | глобальная инфраструктура | один провайдер РФ, были инциденты кластера (~1 час восстановления) |

---

## 2. Лучшие практики приватного хранения фото пользователей

### 2.1 Выдача: presigned GET vs проксирование через бэкенд

**Вариант А: приватный бакет + короткоживущие presigned GET** (бэкенд только авторизует и подписывает).

Плюсы:
- байты фото не идут через бэкенд: экономия трафика/памяти Go-процесса, меньше p99 у API;
- масштабируется естественно — единственный ограничитель, лимит чтений бакета (1000/с у Reg.ru — это уже ~86 млн выдач в сутки);
- S3 отдаёт `Content-Type`, `Content-Length`, `ETag`, `Last-Modified` сам, поддерживает `Range`-запросы.

Минусы и что с ними делать:
- **Кэширование.** Presigned URL — это URL с query-параметрами подписи: браузер и прокси кэшируют его как отдельный ресурс, и срок истечения подписи **не** отзывает уже закэшированную копию — её держит `Cache-Control`. Управление: выставлять объекту при загрузке `Cache-Control: private, max-age=N` так, чтобы `N` был меньше TTL подписи; тогда кэш никогда не переживёт действующую подпись. Разные подписи одного объекта = промахи кэша (ключ кэша включает query) — для одного и того же фото URL будет стабилен, пока бэкенд округляет `Expires` (например, выдаёт подписи, кратные 5 минутам).
- **Отзыв доступа** не мгновенный: ссылка живёт до истечения TTL. При удалении фото из системы объект стоит удалять из бакета сразу (`DeleteObject`) — тогда невыданная ссылка умрёт с 404, а уже выданная — по TTL (минимизируем TTL).
- **Логирование** просмотров идёт не в access-лог приложения. Компенсация: логировать сам факт выдачи ссылки (кто, какое фото, когда) в бэкенде — для аудита персональных данных этого достаточно.
- TTL: у AWS SigV4 максимум 7 дней; рекомендуемый рабочий диапазон для персональных фото — **60–900 секунд** (страница или сессия просмотра). Каждый просмотр галереи = один запрос к бэкенду за пачкой подписей (batch endpoint).

**Вариант Б: проксирование через бэкенд** (`GET /api/.../photos/{id}` → бэкенд скачивает из S3 и стримит).

Плюсы:
- полный контроль на каждый запрос: авторизация, логирование, мгновенный «отзыв», подмена Content-Disposition, счётчики;
- кэширование управляется целиком нами (`Cache-Control` + CDN/redis перед бэкендом), URL стабильны и «вечные»;
- наружу не торчит endpoint хранилища вообще.

Минусы:
- весь трафик фото проходит через Go-процесс: память (буферы/стриминг), горутины, трафик ×2 (S3→бэкенд, бэкенд→клиент);
- latency и стоимость масштабирования растут линейно с трафиком фото;
- каждый просмотр — операция чтения S3 (те же лимиты, плюс нагрузка на бэкенд).

**Рекомендация для Arenda Platform**: гибрид.
- **Превью** (мелкие, генерируются нами при загрузке) — отдавать через бэкенд с агрессивным `Cache-Control: private, max-age=86400, immutable` (URL типа `/api/v1/.../photos/{id}/thumb`) или, если лень городить стриминг, тоже presigned GET с длинным TTL (24 ч) — риск утечки превью ниже.
- **Оригиналы** — только presigned GET с TTL 5–15 минут после серверной авторизации.

### 2.2 Загрузка: presigned PUT из браузера vs через бэкенд

**Вариант А: presigned PUT напрямую из браузера.**
Флоу: фронт запрашивает у бэкенда «место под фото» → бэкенд генерирует ключ (UUID), проверяет квоты/права, подписывает PUT c `Content-Type` и возвращает `{url, key, expiresAt}` → браузер `fetch(url, {method:'PUT', body: file, headers:{'Content-Type': ...}})` → фронт сообщает бэкенду «готово» → бэкенд валидирует и фиксирует ключ в БД.

Плюсы: байты не проходят через бэкенд; горизонтально масштабируется; поддерживается Reg.ru (CORS-методы PUT/POST задокументированы).
Минусы/контроль:
- **CORS обязателен** на бакете (AllowedOrigins = продовый домен, не `*`; методы `PUT`, `HEAD`; заголовок `Content-Type` в `AllowedHeaders`). Preflight OPTIONS должен проходить быстро — иначе UX страдает.
- **Content-Type контролируется подписью**: заголовок `Content-Type` входит в SigV4-подпись — если клиент пришлёт другой, получит `SignatureDoesNotMatch`. Бэкенд подписывает только белый список (`image/jpeg`, `image/png`, `image/webp`).
- **Размер presigned PUT не ограничивает** (`Content-Length` в подпись не включается). Контроль: проверка размера на фронте, **обязательный `HeadObject` после сигнала «готово»** — бэкенд сверяет фактический размер/Content-Type и удаляет объект при нарушении. Радикальный вариант ограничения размера — presigned POST-политика с `content-length-range` (`PresignPostObject` в SDK); поддержка POST-policy на Reg.ru **[ПРОВЕРИТЬ]**.
- **Валидация содержимого** всё равно на бэкенде после загрузки: магические байты + декодирование (см. 2.4). Загруженный «в обход» объект до валидации не показываем никому.
- Ключ известен до загрузки (сгенерирован бэкендом) — имя файла пользователя не участвует в ключе вообще.

**Вариант Б: multipart/form-data через бэкенд.**
Флоу: фронт шлёт файл на бэкенд → бэкенд валидирует (размер, магические байты, декодирование, EXIF) → кладёт в S3.

Плюсы: единая точка валидации до попадания в хранилище; проще доносить до «сирот» (нет объектов без записи в БД); не нужен CORS на бакете; можно обрабатывать (ре-энкод, превью) до сохранения оригинала.
Минусы: трафик ×2 и память на бэкенде; лимиты тела запроса (Next.js/nginx); медленнее для больших файлов.

**Рекомендация**: для фото (обычно < 10–20 МБ) допустимы оба; базовый вариант — **presigned PUT + пост-валидация**, для маленьких аватаров можно упростить до загрузки через бэкенд. Мультичастная загрузка (S3 Multipart) из браузера — оверкилл для фото; бэкенду для файлов > 100 МБ использовать `manager.Uploader` (см. раздел 3), с `LeavePartsOnError=false`, чтобы SDK сам делал `AbortMultipartUpload`.

Незавершённые multipart-загрузки (любого происхождения) — фоновый джоб: `ListMultipartUploads` раз в сутки + `AbortMultipartUpload` для загрузок старше N часов (в AWS это делает lifecycle-правило `AbortIncompleteMultipartUpload`; на Reg.ru см. 1.9 — вручную).

### 2.3 Именование ключей, версионирование, жизненный цикл

- **UUID в ключе, никаких PII**: ключ — `photos/{entityId}/{uuid}.{ext}` (например, `photos/object/7f3a.../c0ffee....jpg`). Имя файла пользователя, email, ID пользователя в ключе не используются (ключи видны в URL и в логах хранилища; ID пользователя в префиксе — спорно: упрощает выборку «все фото пользователя», но утекает внутренний идентификатор в URL — допустимо, если ID не секретен; по умолчанию — UUID-папка на пользователя, если группировка нужна).
- Плоское пространство имён: префиксы — только для жизненного цикла и ListObjects, не для «папок» в UI.
- **Перезапись vs версионирование**: аватар — один объект на ключ, перезапись (старый объект заменяется); фото объекта — неизменяемые ключи (новая загрузка = новый UUID, старое удаляем по soft-delete). Включать S3-версионирование на бакете фото **не обязательно**: оно защищает от случайного удаления, но удваивает storage и требует lifecycle-чистки неактуальных версий (на Reg.ru поддерживается — 1.9). Для record-keeping-продукта защита от «удалил не то» достигается soft delete в БД + отложенное (T+N дней) физическое удаление из бакета. Решение фиксировать осознанно.
- **Сироты**: объект в бакете без записи в БД (загрузили, но не сохранили форму) — джоб сверки: `ListObjectsV2` по префиксу `tmp/`/`photos/` против БД; всё старше 24–48 часов без владельца — удалять. Поэтому загружать сразу в финальный префикс не стоит: presigned PUT выдаётся на ключ `tmp/uploads/{uuid}.{ext}`, после валидации бэкенд делает `CopyObject` в финальный ключ (или просто перемещает запись в БД и меняет префикс — проще держать «сиротный» префикс и чистить его).
- **Удаление**: soft delete в БД → джоб через N дней вызывает `DeleteObject`. Это и защита от ошибок, и соответствие «право на забвение» (фото физически уходит из хранилища, а не только помечается).

### 2.4 Обработка изображений: превью, EXIF, SVG/XSS, сниффинг

- **Превью на сервере**: оригинал не отдаётся в списках — генерируем 1–2 размера (например, 256px и 1024px) при приёмке. Библиотеки Go: `github.com/disintegration/imaging`, `github.com/h2non/bimg` (libvips) или `golang.org/x/image`. Превью хранить рядом (`{key_prefix}/{uuid}_1024.jpg`).
- **EXIF-стриптинг**: JPEG/HEIC-файлы несут EXIF, включая **GPS-координаты места съёмки** — для фото недвижимости и аватаров это утечка адреса/геолокации пользователя. Обязательный минимум: не отдавать оригинал без обработки, либо стрипнуть EXIF. Надёжный способ — **ре-энкод** (декодировать → рисовать заново в новый файл): заодно ломает встроенные вредоносные payload'ы. OWASP прямо рекомендует: «For images, decode and re-encode to an allowed image format, explicitly removing unnecessary metadata», с оговоркой, что ре-энкод не гарантирует 100% чистоты — библиотеку держать обновлённой и ограничивать ресурсы (лимиты размеров изображения — pixel-bomb'ы: `image.DecodeConfig` до декодирования, отказ при > ~30–50 МП).
- **SVG = XSS**: SVG — это XML с активным содержимым (`<script>`, `onload=`, внешние ссылки); при отдаче как `image/svg+xml` браузер исполняет скрипты в контексте origin, на котором открыт файл. Правило: **SVG не принимать и не хранить вообще** (наш продукт — фото); если когда-нибудь понадобится — только после санитизации и с отдачей с отдельного «грязного» origin + `Content-Security-Policy: sandbox`, что для приватного бакета через presigned не устроить.
- **Content-type sniffing**: заголовку `Content-Type` от клиента не верить («Validate the file type, don't trust the Content-Type header as it can be spoofed» — OWASP). В Go: `http.DetectContentType(первые 512 байт)` (реализация WHATWG MIME Sniffing) + `image.DecodeConfig` для подтверждения, что это действительно декодируемое изображение. Отдавать файлы наружу с тем Content-Type, который мы сами определили, и без `X-Content-Type-Options: nosniff`-нарушений (при проксировании бэкенд сам ставит заголовки; при presigned GET хранилище отдаёт то, что сохранили при загрузке, — ещё одна причина подписывать Content-Type при PUT).
- Дополнительно: `Content-Disposition: inline; filename="..."` (без пользовательских имён) при отдаче через бэкенд; лимит распаковки не актуален (не принимаем архивы).

### 2.5 Чеклист безопасности S3-хранилища фото

- [ ] Бакет создан с типом доступа «По ключам»; в политиках нет правил с «Открыт для всех»; после любых манипуляций с типом доступа перепроверить политики (переключение типа сносит правила — 1.4).
- [ ] CORS бакета: `AllowedOrigins` = конкретные продовые домены (без `*`), методы только нужные (`PUT`/`HEAD` для загрузочного бакета, `GET` — если бакет используется и для выдачи), `AllowedHeaders` — по факту (`Content-Type`), не `*` там, где можно перечислить.
- [ ] TTL presigned GET — минуты, не часы/дни; подписи кратные (для попадания в HTTP-кэш).
- [ ] `Cache-Control: private, max-age=<меньше TTL подписи>` выставляется на объект при загрузке; для превью через бэкенд — приватное кэширование.
- [ ] Белый список Content-Type подписан в presigned PUT; `HeadObject`-валидация размера после загрузки; джоб-чистка сирот в `tmp/` и аборт multipart старше N часов.
- [ ] Ключи = UUID без PII; пользовательские имена файлов не попадают ни в ключ, ни в Content-Disposition.
- [ ] Никаких wildcard публичных политик; отдельный набор ключей (Access Key/Secret) на окружение; секреты в env/secret-manager, не в репо; ротация пары ключей по регламенту.
- [ ] Ре-энкод/стрип EXIF (GPS) до первой выдачи; лимит мегапикселей до декодирования; SVG запрещён; Content-Type определяется по магическим байтам, а не по заголовку клиента.
- [ ] Шифрование at rest: серверное шифрование провайдера (для повышенных требований — SSE-C с ключом в KMS/secret-manager; помнить: потеря ключа = потеря данных).
- [ ] HTTPS-only; собственные домены к бакету не подключать (только HTTP — 1.2).
- [ ] Аудит: логировать выдачу подписей и загрузок в приложении; мониторить 4xx/5xx от хранилища.
- [ ] Бэкап/восстановление осознаны: тройная репликация у провайдера не заменяет экспорт (rclone sync на второй бакет/другого провайдера по расписанию), особенно с учётом исторических инцидентов Рег.облака.

---

## 3. Скетч для Go-бэкенда на aws-sdk-go-v2 против Reg.ru S3

### 3.1 Конфигурация клиента

```go
package s3storage

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func NewClient(ctx context.Context, endpoint, region, accessKey, secret string) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region), // Reg.ru: регион можно оставить пустым/фиктивным
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: accessKey, SecretAccessKey: secret}, nil
		})),
	)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint) // https://s3.regru.cloud
		o.UsePathStyle = true                 // https://s3.regru.cloud/<bucket>/<key> — стиль, который документирует Reg.ru
	}), nil
}
```

Существенно: `BaseEndpoint` — рекомендованный способ указать custom endpoint (старый `EndpointResolver` deprecated), `UsePathStyle` — обязательный (документированный Reg.ru формат — path-style). Пресайнер наследует все опции клиента, отдельно endpoint не задаётся.

### 3.2 Выдача presigned GET

```go
type Presigner struct{ presign *s3.PresignClient }

func (p *Presigner) PhotoURL(ctx context.Context, bucket, key string, ttl time.Duration) (string, time.Time, error) {
	req, err := p.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl)) // 5–15 * time.Minute
	if err != nil {
		return "", time.Time{}, err
	}
	return req.URL, time.Now().Add(ttl), nil
}
```

Хендлер API: авторизация → проверка доступа к объекту (владелец/аренда/агент) → batch-выдача подписей для списка фото (`{key, url, expiresAt}[]`). Округлять TTL до кратности (например, полные 5-минутные слоты), чтобы URL не менялся на каждый запрос и попадал в HTTP-кэш.

### 3.3 Выдача presigned PUT на загрузку

```go
func (p *Presigner) UploadURL(ctx context.Context, bucket, key, contentType string, ttl time.Duration) (string, error) {
	req, err := p.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(bucket),
		Key:          aws.String(key), // tmp/uploads/<uuid>.<ext> — UUID генерируем здесь
		ContentType:  aws.String(contentType), // входит в подпись: клиент обязан прислать такой же
		CacheControl: aws.String("private, max-age=600"),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}
```

- `contentType` бэкенд принимает только из белого списка `image/jpeg|image/png|image/webp`.
- Размер лимитируем на фронте + пост-валидацией (3.4). Если Reg.ru поддержит presigned POST-политику — `PresignPostObject` с `content-length-range` закрывает лимит у источника **[ПРОВЕРИТЬ]**.

### 3.4 Приёмка: валидация, метаданные, превью

```go
func AcceptUpload(ctx context.Context, client *s3.Client, bucket, key string, maxBytes int64) (realCT string, err error) {
	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil { return "", err }
	if aws.ToInt64(head.ContentLength) > maxBytes {
		return "", errors.New("too large")
	}
	rng := &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}
	// ...
	obj, err := client.GetObject(ctx, rng)
	if err != nil { return "", err }
	defer obj.Body.Close()

	head512 := make([]byte, 512)
	n, _ := io.ReadFull(obj.Body, head512)
	realCT = http.DetectContentType(head512[:n]) // магические байты, не доверяем заголовку
	switch realCT {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return "", fmt.Errorf("unsupported content type %q", realCT)
	}
	// Декодирование с лимитом: image.DecodeConfig(io.MultiReader(bytes.NewReader(head512[:n]), obj.Body))
	// → проверка W*H <= 50MP; затем ре-энкод (стрип EXIF/GPS) и генерация превью,
	// результат — PutObject в финальный префикс photos/..., затем DeleteObject tmp-ключа.
	return realCT, nil
}
```

Показывать фото пользователю можно только после этой стадии; невалидные объекты — удалять сразу.

### 3.5 Большие файлы через бэкенд: multipart

Для фото не нужно, но если появятся видео/большие документы — `github.com/aws/aws-sdk-go-v2/feature/s3/manager`:

```go
up := manager.NewUploader(client, func(u *manager.Uploader) {
	u.PartSize = 16 << 20 // 16 MiB
	u.Concurrency = 4
	u.LeavePartsOnError = false // SDK сам вызовет AbortMultipartUpload при ошибке
})
_, err := up.Upload(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: file})
```

Плюс суточный джоб: `ListMultipartUploads` → `AbortMultipartUpload` для загрузок старше 24 ч (на Reg.ru lifecycle-правило для этого не документировано — 1.9).

### 3.6 Загрузочный CORS и выдача на фронте (Next.js)

- CORS бакета (прод): `AllowedOrigins: ["https://<домен приложения>"]`, `AllowedMethods: ["PUT","HEAD"]` (+`GET`, если бакет и раздаёт), `AllowedHeaders: ["Content-Type"]`, `ExposeHeaders: ["ETag"]`.
- Фронт: `PUT` на presigned URL с тем же `Content-Type`, что вернул бэкенд; затем `POST /api/.../uploads/{id}/complete`; в `<img src>` — presigned GET URL из batch-endpoint, обновлять перед истечением.

### 3.7 Удаление

```go
_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
```

Soft delete в БД → отложенный джоб; при желании — включить версионирование бакета и правило `NoncurrentVersionExpiration` (1.9) как страховку от случайного удаления.

---

## Источники

Reg.ru / Рег.облако (официальная документация и продукт):

1. Объектное хранилище S3 — страница продукта (тарифы, матрица возможностей, FAQ: классы хранения, подписанные URL, шифрование): https://reg.cloud/services/s3-storage
2. Способы доступа к файлам в S3 (типы доступа, path-style ссылка `https://s3.regru.cloud/bucket/object`, presign, TTL 3600/`--expires-in`): https://reg.cloud/support/instrukcii/obektnoe-hranilishe-s3/sposoby-dostupa-k-faylam-v-s3
3. Установка и настройка AWS CLI (S3 API Endpoint и ключи в панели, `aws configure`, `endpoint_url`): https://reg.cloud/support/instrukcii/obektnoe-hranilishe-s3/ustanovka-i-nastrojka-aws-cli
4. Настройка CORS для доступа к объектам S3 (put/get/delete-bucket-cors, поддерживаемые методы, пример): https://reg.cloud/support/instrukcii/obektnoe-hranilishe-s3/nastrojka-cors-dlya-dostupa-k-obektam-s3
5. SSE-C в хранилище S3 (AES256, поддерживаемые операции, заголовки): https://reg.cloud/support/instrukcii/obektnoe-hranilishe-s3/sse-c-v-hranilishe-s3
6. Версионирование объектов в хранилище S3 (включение, версии, lifecycle `NoncurrentVersionExpiration`): https://reg.cloud/support/instrukcii/obektnoe-hranilishe-s3/versionirovanie-obektov-v-hranilishe-s3
7. Удаление частично загруженного объекта (abort-multipart-upload, list-multipart-uploads): https://reg.cloud/support/instrukcii/obektnoe-hranilishe-s3/udalenie-chastichno-zagruzhennogo-objekta
8. Подключение собственного домена к бакету (website-хостинг, публичный бакет, отсутствие TLS): https://reg.cloud/support/instrukcii/obektnoe-hranilishe-s3/podklyuchenie-sobstvennogo-domena-k-baketu-v-hranilishe-s3
9. Перенос данных в объектное хранилище S3 (rclone, endpoint `https://s3.regru.cloud`, SigV4, ограничение переноса политик): https://reg.cloud/support/instrukcii/obektnoe-hranilishe-s3/perenos-dannyh-v-obektnoe-hranilishe-s3
10. Заказ и управление объектным хранилищем S3 (создание бакета, квота 10 ГБ, типы доступа): https://reg.cloud/support/servery-vps/obyektnoye-khranilishche-s3/zakaz-i-upravlenie-uslugoj-obektnoe-hranilishche-s3/zakaz-i-upravleniye-obyektnym-khranilishchem-s3
11. Ограничения на количество операций чтения и записи (1000/500 на бакет, 10000/5000 на пользователя, >10 Гбит/с): https://reg.cloud/support/servery-vps/obyektnoye-khranilishche-s3/zakaz-i-upravlenie-uslugoj-obektnoe-hranilishche-s3/ogranicheniya-na-kolichestvo-operacij-chteniya-i-zapisi-v-hranilishe-s3
12. Политики доступа в хранилище S3 (типы доступа, правила, области действия, условия, default deny, сброс политик при смене типа): https://reg.cloud/support/servery-vps/obyektnoye-khranilishche-s3/zakaz-i-upravlenie-uslugoj-obektnoe-hranilishche-s3/politiki-dostupa-v-hranilishe-s3
13. Управление ключами доступа в хранилище S3 (наборы ключей, endpoint+Access Key+Secret, ротация): https://reg.cloud/support/servery-vps/obyektnoye-khranilishche-s3/zakaz-i-upravlenie-uslugoj-obektnoe-hranilishche-s3/upravlenie-klyuchami-dostupa-v-hranilishe-s3

AWS (семантика SigV4/presigned, применимая к S3-совместимому хранилищу):

14. Download and upload objects with presigned URLs (bearer-токены, максимум 7 дней SigV4, истечение при отзыве креденшелов): https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html
15. Uploading objects with presigned URLs (presigned PUT, подписанный Content-Type): https://docs.aws.amazon.com/AmazonS3/latest/userguide/PresignedUrlUploadObject.html

Безопасность загрузок:

16. OWASP File Upload Cheat Sheet (allowlist расширений, magic bytes, генерация имени сервером, ре-энкод изображений и стрип метаданных, лимиты): https://cheatsheetseries.owasp.org/cheatsheets/File_Upload_Cheat_Sheet.html

aws-sdk-go-v2:

17. S3-пакет (PresignClient, PresignGetObject/PresignPutObject, WithPresignExpires, наследование endpoint/креденшелов): https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3
18. feature/s3/manager (Uploader: PartSize/Concurrency/LeavePartsOnError, авто-abort; deprecated в пользу transfermanager): https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/manager
19. Исходник service/s3/options.go (поля `BaseEndpoint`, `UsePathStyle` с doc-комментариями): https://github.com/aws/aws-sdk-go-v2/blob/main/service/s3/options.go

Кэширование presigned URL:

20. Cacheable S3 signed URLs (Cache-Control vs срок подписи, кэш не переживает подпись при коротком max-age): https://advancedweb.hu/cacheable-s3-signed-urls/
21. AWS re:Post — Troubleshoot expiration of presigned URL (когда подпись истекает раньше заявленного): https://repost.aws/knowledge-center/presigned-url-s3-bucket-expiration

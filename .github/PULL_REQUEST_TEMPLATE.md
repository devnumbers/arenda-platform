## Чеклист

- [ ] CI зелёный (lint, тесты, миграции up/down, сборка и сканеры образов)
- [ ] Миграции backward-compatible по expand-contract — новая схема работает с предыдущей версией кода (см. `docs/adr/0024-deploy-pipeline-ghcr-runner.md`)
- [ ] Destructive-изменения схемы (DROP/rename/ужесточение constraints) вынесены в отдельный релиз — вторым шагом, после того как старый код полностью ушёл
- [ ] `CREATE INDEX CONCURRENTLY` для больших таблиц (миграция без транзакции)
- [ ] При изменении набора env-переменных: обновлены `deploy/.env.stage.example` / `deploy/.env.prod.example`, а секрет `ENV_FILE` в GitHub Environments синхронизирован — иначе deploy упадёт на сверке ключей (см. `docs/deployment.md`)
- [ ] Документация обновлена (`CONTEXT.md` / `docs/` / ADR при изменении домена или архитектуры)

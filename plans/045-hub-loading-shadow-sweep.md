# 045 — Системная зачистка: снос теневых loading.tsx остальных хабов

- **Status**: DONE
- **Baseline**: план 044 (объекты DONE)
- **Severity**: MEDIUM (решение владельца 16.10: «давай сразу все»)
- **Origin**: карта #1437, догонка feel-check владельца; болезнь обнаружена аудитом плана 044
- **Estimated scope**: снос 5 loading.tsx, e2e, ~40 строк удалений

## Problem

Та же болезнь, что у объектов (план 044, дефект 2), системно: segment-
level `loading.tsx` хаба стримится первым кадром холодной загрузки всех
его детей (async-компонент страницы ребёнка suspends до inner Suspense —
внешний boundary рисует родительский архетип). Список детей с серверным
префетчем (аудит 16.10):

- `/participants/loading.tsx` → тень на `/participants/[participantId]`
  (in-page ParticipantLoading есть ✓);
- ~~`/tasks/loading.tsx`~~ — **аудит при исполнении: тени НЕТ** — у
  `/tasks/[ruleId]/edit` нет серверного префетча (файл не в списке
  ServerPrefetchBoundary), страничный компонент не suspends над своей
  Suspense-границей, родительский loading.tsx на холодную загрузку детей
  не влияет → файл остаётся (та же логика, что у operations);
- `/operations/loading.tsx` — детей с префетчем нет, кадр хаба
  (OperationsLoading хромирован) — остаётся;
- `/history`, `/profile` — segment loading.tsx нет, проблемы нет.

Снос в e2e покрывают `/payments/due` и `/participants/[participantId]`
(ид участника детерминирован сидом); дети контактов и уведомлений в сиде
не создаются — их снос накрывается тем же механизмом, гардом хаба
(«Контакты» в спеке) и полным прогоном. Все loading-виджеты хабов
хромированы (TopNav + PageContent, «кнопки в покой») — выбившийся
PropertiesLoading чинится планом 044. Все хабы имеют in-page Suspense
fallback — снос сегментного файла не меняет кадр холодной загрузки самого
хаба (тот же виджет через in-page границу) и не трогает клиентскую
навигацию (RSC-стрим несёт тот же fallback).

Решение владельца 16.10: зачищать сразу все хабы.

## Target

**Шаг 1 — снос четырёх файлов**: `app/(screens)/{contacts,payments,
notifications,participants}/loading.tsx` — те же причины, что у
properties/loading.tsx в плане 044 (шаг 3). operations/loading.tsx и
tasks/loading.tsx остаются (детей с серверным префетчем нет — файлы ничью
загрузку не затеняют и обслуживают холодную загрузку хаба тем же кадром,
что и in-page fallback).

**Шаг 2 — эмпирика tasks/[ruleId]/edit**: после сноса родителя холодная
загрузка edit стримит TaskEditLoading (собственный loading.tsx ребёнка
становится ближайшим boundary). Если прогон покажет обратное (пустой кадр
до резолва) — обернуть content edit-страницы в Suspense fallback
TaskEditLoading по прецеденту детей /properties; факт зафиксировать в
плане.

**Шаг 3 — grep-проход**: импортов снесённых виджетов в снесённых файлах
нет нигде больше (TasksLoading остаётся в tasks/page.tsx fallback и
index.ts; PaymentsLoading — в payments/page.tsx; и т.д. — они нужны как
in-page кадры, не трогаются).

## TDD seams

- Новый `e2e/hub-cold-loading.spec.ts` (CDP-троттлинг, как в 044):
  холодная загрузка `/payments/due` и `/participants/[participantId]`
  (ид из сида) — MutationObserver-журнал: тестид родительского скелетона
  не появляется ни разу (сейчас: появляется — красный); гард «страница
  доехала» — заголовок архетипа. Третий тест — гард регрессии сноса:
  холодная загрузка самого хаба «Контакты» стримит свой кадр (после
  сноса его рисует in-page fallback). Маркеры — корневые тестиды,
  добавленные четырём loading-виджетам (contacts-loading,
  payments-loading, notifications-loading, participants-list-loading):
  подводных камней два, пойманы при исполнении — addInitScript
  сериализует только тело функции (маркеры инлайн), и замер геометрии до
  загрузки CSS не имеет смысла (044).
- Существующие e2e хабов — полный прогон в финале.

## Boundaries

- Do NOT трогать loading-виджеты, кроме добавления корневых тестидов
  (все хромированы, канон §7; тестиды — только крючки e2e-журнала).
- Do NOT трогать operations/loading.tsx и tasks/loading.tsx (детей с
  серверным префетчем нет) и history/profile (проблемы нет).
- Do NOT трогать in-page fallback'и и client-side навигацию — снос файла
  меняет только кадр холодной загрузки детей.
- Do NOT трогать properties (план 044 уже снёс их файл).

## Verification

- `npx tsc --noEmit`, `npm run lint`, `make frontend-test` — зелёные.
- `make frontend-e2e TESTS=-g холодная` + полный прогон e2e в финале
  догонки — зелёные.
- Feel на стенде: холодные загрузки контакта, due, уведомления, участника —
  чужого скелетона нет, архетипный кадр с хромом.

## Done when

Ни один префетчащий ребёнок хаба не получает скелетон родительского
списка на холодной загрузке; сегментные loading.tsx снесены там, где есть
префетчащие дети (contacts, payments, notifications, participants);
operations и tasks сохранены (детей нет); e2e зелёные.

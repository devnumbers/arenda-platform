# 034 — Круг фото без мёртвого воздуха: глиф-подложка + проявление по onLoad

- **Status**: TODO
- **Commit**: 40d303a87
- **Severity**: HIGH
- **Category**: Feedback / фото-канон отображения (ADR 0065, фолбэк #1275)
- **Estimated scope**: 1 новый примитив design-слоя + 2 канон-компонента + тесты

## Problem

Все пользовательские фото — raw `<img>` в фиксированных кругах (ADR 0065,
18 сайтов, канон фолбэка #1275). Состояния загрузки нет нигде: атрибут `onLoad=` не используется по всему
`apps/frontend` (0 совпадений; подстрочные `isActionLoading`/
`DirectionLoading` — не про img), глиф рисуется
только когда фото отсутствует или битое:

```tsx
/* PropertyAvatar.tsx:102-111 — UserAvatar.tsx:58-67 то же */
{photoUrl && !photoBroken ? (
  <img
    src={photoUrl}
    alt=""
    className="h-full w-full object-cover"
    onError={() => setPhotoBroken(true)}
  />
) : (
  <Glyph className={cn('text-[#D3D7D9]', SURFACE_ICON[surface])} aria-hidden />
)}
```

Пока байты летят (холодный вход, приватный стрим `max-age=300` мимо кеша,
мобильная сеть), круг — пустой фон: «мёртвый воздух», вопрос тикета #1423.
Скелетон-фаза в круге отклонена (решение-предложение, README раздела #1423):
на кеш-попадании она мигает ровно тем, чего вопрос тикета опасается.

Паттерн «глиф под фото + проявление» руками повторён в 18 местах —
новая механика (onLoad-гейт) без примитива разъехалась бы в 18 копий.

## Target

Примитив `CirclePhoto` в design-слое — содержимое клипающего круга:
глиф-подложка живёт всегда (ожидание и ошибка), фото скрыто до onLoad
и проявляется на токенах §8; `decoding="async"` запечён (план 035
раскатывает); гард пустой строки сохранён (WebKit: `<img src="">` не
стреляет onError — канон UserAvatar, `history-feed-screen.tsx:760-762`).

```tsx
/* shared/ui/design/circle-photo.tsx — эскиз, TDD ведёт сигнатуру */
export type CirclePhotoProps = {
  /** Стриминговый путь (ADR 0065); null/undefined/'' — только глиф. */
  readonly photoUrl?: string | null;
  /** Глиф-подложка: тип объекта / BoldUser — выбирает вызывающий. */
  readonly glyph: JSX.Element;
  readonly className?: string;
};

export function CirclePhoto({ photoUrl, glyph, className }: CirclePhotoProps): JSX.Element {
  const [photoBroken, setPhotoBroken] = useState(false);
  const [photoReady, setPhotoReady] = useState(false);
  const showPhoto = photoUrl != null && photoUrl !== '' && !photoBroken;
  return (
    <>
      {glyph}
      {showPhoto && (
        <img
          src={photoUrl}
          alt=""
          decoding="async"
          className={cn(
            'absolute inset-0 h-full w-full object-cover transition-opacity duration-(--dl-duration-fast) ease-(--dl-ease)',
            photoReady ? 'opacity-100' : 'opacity-0',
            className,
          )}
          onLoad={() => setPhotoReady(true)}
          onError={() => setPhotoBroken(true)}
        />
      )}
    </>
  );
}
```

Родитель обязан держать `relative`/клип (сегодняшние круги уже такие:
`overflow-hidden rounded-full` + центрирование глифа — структура не меняется,
`img` получает `absolute inset-0`). Канон-компоненты переходят на примитив,
поверхности и глифы остаются у них:

```tsx
/* PropertyAvatar.tsx — внутренности клипающего span */
<CirclePhoto
  photoUrl={photoUrl}
  glyph={<Glyph className={cn('text-[#D3D7D9]', SURFACE_ICON[surface])} aria-hidden />}
/>
```

Семантика жизни флагов — как сегодня, записанная в докблоках:
`photoBroken` до размонтирования (списки кейуются по id); `photoReady`
не сбрасывается при смене src (свап `?v=N` на живом элементе — старый
кадр держится до декода нового, инвариант бастера, план 038): onLoad
стреляет повторно, скрытия нет. Переход null→url (фото добавили) — img
монтируется свежим, проявляется с глифа; url→null — круг к глифу (осознанно,
канон staged-удаления).

## Repo conventions to follow

- Движение — `DESIGN.md` §8: `--dl-duration-fast` (250ms) + `--dl-ease`
  (`tokens.css:101-103`); reduced-motion §8 укорачивает сам.
- Фолбэк и докблоки — `user-avatar.tsx:9-12,46-47`,
  `PropertyAvatar.tsx:85-88` (переносятся в примитив дословно).
- Тестовый стиль — `shared/ui/design/button.test.ts`.
- ADR 0065 — `docs/adr/0065-private-photos-one-image-per-entity.md`.

## Steps

1. TDD: `shared/ui/design/circle-photo.test.tsx`: (a) без photoUrl — один
   глиф, img нет; (b) `''`/null/undefined — глиф (гард WebKit); (c) с
   photoUrl — глиф в DOM, img с `opacity-0` и `decoding="async"`;
   (d) `fireEvent.load(img)` — `opacity-100`; (e) `fireEvent.error(img)` —
   img ушёл, глиф остался. Сначала красный, затем шаг 2.
2. `circle-photo.tsx` — Target-код; экспорт из `shared/ui/design/index.ts`.
3. PropertyAvatar — внутренности клипающего span на примитив; глиф и
   поверхности не трогаются; устаревший фрагмент докблока про флаг
   (`PropertyAvatar.tsx:85-88`) сверить с фактической жизнью флага.
4. UserAvatar — то же (`user-avatar.tsx:58-67`).
5. `make frontend-test`; визуальная проба /ui-kit не требуется — приёмка
   feel-check'ом раздела #1423 на железе.

## Boundaries

- Do NOT трогать остальные 16 raw-сайтов — план 035.
- Do NOT вводить blur/скелетон/аспект-резервацию — отклонены разделом.
- Do NOT менять бастер и хуки чтения (`photoDisplayUrl`, бастер-ключи).
- Родитель без `relative` — примитив не чинит: STOP и доложить (сегодня
  все круги клипающие — таких быть не должно).
- Если код канон-компонентов не совпал с цитатами — STOP и доложить.

## Verification

- **Mechanical**: новый тест примитива — зелёный; `make frontend-test` —
  зелёный.
- **Feel check** (раздел #1423): throttled-сеть — круги с глифом вместо
  пустоты, фото проявляется без скачка; кеш-попадание — без миганий;
  замена фото — старый кадр держится до нового.
- **Done when**: PropertyAvatar и UserAvatar не содержат собственных
  `onError`-img; 0 совпадений «пустого ожидания» на двух канонах.

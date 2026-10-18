# 004 — Кроссфейд шагов без провала альфы

- **Status**: DONE
- **Commit**: 613729c87
- **Severity**: MEDIUM
- **Category**: Cohesion/crossfade (обе плёнки с альфой гаснут одновременно — в середине свопа суммарное покрытие ~75%, фон просвечивает сквозь мокап)
- **Estimated scope**: 1 файл (components/sections/steps.tsx)

## Problem

Все 4 мокапа в стеке гаснут/проявляются одновременно:

```tsx
// components/sections/steps.tsx:95-108 — current
{STEPS.map((item, index) => (
  <Image
    key={item.text}
    src={item.image}
    alt={item.alt}
    width={273}
    height={400}
    sizes="273px"
    priority={false}
    className={`absolute inset-0 transition-opacity duration-[600ms] ease-inout ${
      active === index ? "opacity-100" : "opacity-0"
    }`}
  />
))}
```

Ассеты с альфа-каналом (проверено: VP8X alpha-флаг 0x10 во всех четырёх `assets/sections/step-*.webp`) — в середине кроссфейда под прозрачными местами мокапа просвечивает градиент подложки.

## Target

Уходящий слой НЕ гаснет: держится полностью непрозрачным под въезжающим верхним. Анимируется только входящий. Провал покрытия исчезает по построению (1 + a ≥ 1).

```tsx
// target
const [prev, setPrev] = useState<number | null>(null);

const selectStep = (index: number) => {
  if (index === active) {
    return;
  }
  setPrev(active);
  setActive(index);
  swapTextStep(index);
};

// в разметке стека:
{STEPS.map((item, index) => {
  const isActive = active === index;
  const isPrev = prev === index;
  return (
    <Image
      key={item.text}
      src={item.image}
      alt={item.alt}
      width={273}
      height={400}
      sizes="273px"
      priority={false}
      className={`absolute inset-0 ${
        isActive
          ? "z-10 opacity-100 transition-opacity duration-[600ms] ease-inout"
          : isPrev
            ? "z-0 opacity-100"
            : "z-0 opacity-0"
      }`}
    />
  );
})}
```

Механика: новый active въезжает 0→1 с transition поверх prev (1, без transition); старый prev мгновенно садится в 0 без transition (он под двумя слоями — невидим).

## Repo conventions to follow

- Кроссфейд маской/подложкой — не требуется; визуал покоя и середины свопа 1:1 макету (мокап в покое и так полностью непрозрачен над градиентом).

## Steps

1. steps.tsx: добавить `const [prev, setPrev] = useState<number | null>(null);`, дополнить `selectStep` (Target).
2. Заменить className-логику стека изображений (95-108) на Target-логику.

## Boundaries

- Do NOT трогать тайминги (600мс ease-inout остаются), текстовую машину (use-swap-phase), подложку-градиент.
- Do NOT менять размеры/sizes/priority изображений.

## Verification

- **Mechanical**: `npm run lint && npm run typecheck && npm run build` — зелёные.
- **Feel check**: клики по шагам 1→2→3→4→2 на 375/1440 — мокап в середине свопа непрозрачен, градиент сквозь него не просвечивает; быстрый спам-клик — без мигания (старый-старый слой садится незаметно).
- Скриншот-паритет покоя до/после (каждый шаг) — байт-в-байт или диф ≤1/255.
- **Done when**: провал альфы не наблюдается на середине свопа, покой 1:1.

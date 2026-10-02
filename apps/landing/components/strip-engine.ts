import { easeOut } from "@/components/carousel-ease";

// Движок лент-каруселей лендинга — канон карусели
// pay.yandex.ru/business/acquiring (замер 02.10): единственная позиция p
// в translate3d дорожки, нативного скролла нет; программные переходы —
// фиксированные 360мс ease-out (на ширинах <720 — 460мс, мобильный
// брейкпоинт той секции) независимо от дистанции, посадка точно в
// позицию покоя. Drag (мышь и тач) — 1:1 за указателем без инерции; на
// отпускании снап на ближайшую позицию, быстрый бросок добавляет одну
// карточку по направлению (если порог половины шага тягой не взят); за
// краями — резиновая оттяжка и возврат. Shift+Scroll и горизонтальный
// трекпад — ровно один шаг на событие, события во время анимации
// глотаются; вертикальное колесо скроллит страницу. Захват указателя в
// полёте перебивает переход и подхватывает ленту с текущей позиции.
// Конечная лента: набор позиций покоя — k×step, последняя прижимает
// правый край стрипа к правому краю контентной колонки.
// prefers-reduced-motion — переходы мгновенные.
//
// Отличие от исходной карусели аренды: захват указателя отложен до
// первых 5px протяжки — тап без движения не перехватывает клик, поэтому
// интерактив внутри карточек (переключатель биллинга в тарифах) работает
// как обычно; реальная протяжка перехватывает его (при setPointerCapture
// click ретаргечится на окно ленты). pointerup/cancel слушаются на
// window — отпуск вне ленты до активации drag не теряется.
//
// Ширина стрипа и шаг позиций покоя измеряются по живым детям дорожки
// (обёртки карточек или сетка тарифов), не по константам — движок не
// знает состав ленты. Позиция без JS = p=0: стартовую геометрию кладёт
// CSS дорожки (паддинг у левого края контентной колонки).

// Длительность переходов — замер секции acquiring Яндекса: 360ms, на
// её мобильном брейкпоинте (<720px) — 460ms.
const STEP_MS = 360;
const STEP_MS_NARROW = 460;
const NARROW_MAX = 720;
// Затухание протяжки за краем: замер Яндекса — сильный жест уводит
// полосу за край примерно на 80px при сырой протяжке ~230px.
const RUBBER = 0.35;
// Порог броска по скорости, px/мс.
const FLICK_V = 0.5;

// Порог активации drag: до него жест считается тапом — класс dragging
// не включается, capture не ставится, клик уходит по назначению.
const DRAG_ACTIVATE_PX = 5;

type StripTier = {
  gap: number;
  // Контентная колонка в текущем окне ленты — задаёт ход (maxP = ширина
  // стрипа − колонка, последняя позиция прижимает стрип к её правому
  // краю). У аренды: 1000px на ПК, ширина окна − 48 на тач-ярусе.
  column: (viewportWidth: number) => number;
};

type StripEngineConfig = {
  tier: () => StripTier;
  // Класс на окне ленты во время drag (курсор/запрет выделения) —
  // зеркалится в CSS-модуле владельца ленты.
  draggingClass?: string;
};

// Мутабельное состояние ленты — живёт в замыкании движка, меняется
// только в обработчиках (канон refs лендинга: никакого доступа в рендере).
type EngineState = {
  p: number;
  anim: { from: number; to: number; t0: number; dur: number } | null;
  raf: number;
  // armed: pointerdown записан, drag ещё не активирован (до 5px).
  armed: boolean;
  dragging: boolean;
  dragMoved: boolean;
  pointerId: number | null;
  dragStartX: number;
  dragStartP: number;
  dragSamples: Array<{ t: number; x: number }>;
};

// Позиции покоя: k-й шаг прижат к контентной колонке; последняя позиция
// — правый край стрипа у правого края колонки (maxP) — шаг длину стрипа
// нацело не делит, как у референса.
function model(
  viewport: HTMLElement,
  track: HTMLElement,
  config: StripEngineConfig,
) {
  const tier = config.tier();
  const children = Array.from(track.children) as HTMLElement[];
  const widths = children.map((child) => child.getBoundingClientRect().width);
  const stripW =
    widths.reduce((sum, w) => sum + w, 0) +
    tier.gap * Math.max(children.length - 1, 0);
  const step = (widths[0] ?? 0) + tier.gap;
  const maxP = Math.max(stripW - tier.column(viewport.clientWidth), 0);
  const pos: number[] = [];
  for (let k = 0; k * step < maxP - 0.5; k += 1) {
    pos.push(k * step);
  }
  if (pos.length === 0 || maxP - (pos[pos.length - 1] ?? 0) > 0.5) {
    pos.push(maxP);
  }
  return { pos, maxP };
}

// Позиция покоя, ближайшая к p, — по модели, не по живому лэйауту.
function nearestIndex(p: number, pos: readonly number[]): number {
  let best = 0;
  let bestDist = Math.abs((pos[0] ?? 0) - p);
  for (let i = 1; i < pos.length; i += 1) {
    const d = Math.abs((pos[i] ?? 0) - p);
    if (d < bestDist) {
      bestDist = d;
      best = i;
    }
  }
  return best;
}

export function createStripEngine(
  viewport: HTMLElement,
  track: HTMLElement,
  config: StripEngineConfig,
) {
  const st: EngineState = {
    p: 0,
    anim: null,
    raf: 0,
    armed: false,
    dragging: false,
    dragMoved: false,
    pointerId: null,
    dragStartX: 0,
    dragStartP: 0,
    dragSamples: [],
  };
  let reduced = false;
  let resizeRaf = 0;

  // Источник истины — st.p; один проход отрисовки кладёт её в дорожку.
  const render = () => {
    track.style.transform = `translate3d(${(-st.p).toFixed(2)}px,0,0)`;
  };

  const frameLoop = () => {
    const stepFn = (t: number) => {
      const a = st.anim;
      if (!a) {
        return;
      }
      const x = a.dur <= 0 ? 1 : Math.min(Math.max((t - a.t0) / a.dur, 0), 1);
      st.p = a.from + (a.to - a.from) * easeOut(x);
      render();
      if (x < 1) {
        st.raf = requestAnimationFrame(stepFn);
      } else {
        st.anim = null;
      }
    };
    st.raf = requestAnimationFrame(stepFn);
  };

  // Единственная анимация движения: фиксированная длительность ease-out
  // до точной позиции покоя, независимо от дистанции.
  const animateTo = (to: number) => {
    if (Math.abs(st.p - to) < 0.5) {
      st.anim = null;
      st.p = to;
      render();
      return;
    }
    const narrow = window.matchMedia(`(max-width: ${NARROW_MAX - 1}px)`).matches;
    st.anim = {
      from: st.p,
      to,
      t0: performance.now(),
      dur: reduced ? 0 : narrow ? STEP_MS_NARROW : STEP_MS,
    };
    cancelAnimationFrame(st.raf);
    frameLoop();
  };

  // Один шаг в направлении dir: соседняя позиция покоя от ближайшей.
  const stepBy = (dir: number) => {
    const { pos } = model(viewport, track, config);
    const cur = nearestIndex(st.p, pos);
    const target = Math.min(Math.max(cur + dir, 0), pos.length - 1);
    animateTo(pos[target] ?? st.p);
  };

  // Стартовая позиция (первый вид макета — p=0 без JS) и пересборка на
  // ресайз: ближайшая карточка встаёт ровно на свою позицию покоя.
  const measure = () => {
    st.anim = null;
    cancelAnimationFrame(st.raf);
    const { pos } = model(viewport, track, config);
    st.p = pos[nearestIndex(st.p, pos)] ?? 0;
    render();
  };

  const onResize = () => {
    cancelAnimationFrame(resizeRaf);
    resizeRaf = requestAnimationFrame(() => {
      if (!st.dragging) {
        measure();
      }
    });
  };

  const onPointerDown = (e: PointerEvent) => {
    if (e.pointerType === "mouse" && e.button !== 0) {
      return;
    }
    if (model(viewport, track, config).maxP <= 0) {
      return; // лента влезает в колонку — двигать нечего
    }
    st.armed = true;
    st.dragging = false;
    st.dragMoved = false;
    st.pointerId = e.pointerId;
    st.dragStartX = e.clientX;
    st.dragStartP = st.p;
    st.dragSamples = [{ t: performance.now(), x: e.clientX }];
    st.anim = null; // жест перехватывает движение
    cancelAnimationFrame(st.raf);
  };

  const onPointerMove = (e: PointerEvent) => {
    if (!st.armed || e.pointerId !== st.pointerId) {
      return;
    }
    const dx = e.clientX - st.dragStartX;
    if (Math.abs(dx) > DRAG_ACTIVATE_PX) {
      st.dragMoved = true;
    }
    if (!st.dragging) {
      if (Math.abs(dx) <= DRAG_ACTIVATE_PX) {
        return;
      }
      st.dragging = true;
      if (config.draggingClass) {
        viewport.classList.add(config.draggingClass);
      }
      viewport.setPointerCapture(e.pointerId);
    }
    const { maxP } = model(viewport, track, config);
    const raw = st.dragStartP - dx;
    // резиновый край: за пределами позиций полоса следует с затуханием
    st.p =
      raw < 0 ? raw * RUBBER : raw > maxP ? maxP + (raw - maxP) * RUBBER : raw;
    render();
    st.dragSamples.push({ t: performance.now(), x: e.clientX });
    if (st.dragSamples.length > 6) {
      st.dragSamples.shift();
    }
  };

  const release = (e: PointerEvent, flick: boolean) => {
    if (!st.armed || e.pointerId !== st.pointerId) {
      return;
    }
    st.armed = false;
    st.pointerId = null;
    if (st.dragging) {
      st.dragging = false;
      if (config.draggingClass) {
        viewport.classList.remove(config.draggingClass);
      }
    }
    if (!st.dragMoved) {
      return; // тап без движения — ничего не делает
    }
    const { pos } = model(viewport, track, config);
    let best = nearestIndex(st.p, pos);
    if (flick) {
      // скорость по окну ~100мс: последний сэмпл к моменту up почти
      // всегда стоит на месте, оконная скорость ловит бросок даже при
      // остановившемся указателе
      const now = performance.now();
      const windowStart = now - 100;
      const last = st.dragSamples.at(-1);
      const ref = st.dragSamples.find((smp) => smp.t >= windowStart);
      if (last && ref && last.t >= windowStart) {
        const dt = Math.max(now - ref.t, 1);
        const vP = -(e.clientX - ref.x) / dt; // скорость p, px/мс
        // бросок двигает ленту на одну карточку, только если тягой
        // порог половины шага не взят (иначе ближайшая уже следующая)
        if (
          Math.abs(vP) > FLICK_V &&
          best === nearestIndex(st.dragStartP, pos)
        ) {
          best = Math.min(Math.max(best + Math.sign(vP), 0), pos.length - 1);
        }
      }
    }
    animateTo(pos[best] ?? st.p);
  };

  const onPointerUp = (e: PointerEvent) => release(e, true);
  const onPointerCancel = (e: PointerEvent) => release(e, false);

  // Shift+Scroll и горизонтальный трекпад — ровно один шаг на событие;
  // события во время анимации или drag глотаются (не в очередь) — серия
  // подряд даёт один шаг, как у референса. Вертикальное колесо не наша
  // ось — скроллит страницу.
  // Нативный drag-and-drop (картинки, выделенный текст, ссылки) забирает
  // указатель себе — Chromium даёт pointercancel и жест умирает на первом
  // же движении. Полоса — жестовая поверхность: dragstart гасится целиком
  // (перетаскивать из неё нечего), как и в каноничной ленте Яндекса.
  const onDragStart = (e: DragEvent) => {
    e.preventDefault();
  };

  const onWheel = (e: WheelEvent) => {
    if (Math.abs(e.deltaX) <= Math.abs(e.deltaY)) {
      return;
    }
    e.preventDefault();
    if (st.anim || st.dragging) {
      return;
    }
    stepBy(e.deltaX > 0 ? 1 : -1);
  };

  const rm = window.matchMedia("(prefers-reduced-motion: reduce)");
  const onRmChange = () => {
    reduced = rm.matches;
  };
  reduced = rm.matches;

  rm.addEventListener("change", onRmChange);
  viewport.addEventListener("pointerdown", onPointerDown);
  viewport.addEventListener("pointermove", onPointerMove);
  window.addEventListener("pointerup", onPointerUp);
  window.addEventListener("pointercancel", onPointerCancel);
  viewport.addEventListener("dragstart", onDragStart);
  viewport.addEventListener("wheel", onWheel, { passive: false });
  window.addEventListener("resize", onResize);
  measure();

  return {
    destroy() {
      rm.removeEventListener("change", onRmChange);
      viewport.removeEventListener("pointerdown", onPointerDown);
      viewport.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerup", onPointerUp);
      window.removeEventListener("pointercancel", onPointerCancel);
      viewport.removeEventListener("dragstart", onDragStart);
      viewport.removeEventListener("wheel", onWheel);
      window.removeEventListener("resize", onResize);
      cancelAnimationFrame(resizeRaf);
      cancelAnimationFrame(st.raf);
      st.anim = null;
      if (config.draggingClass) {
        viewport.classList.remove(config.draggingClass);
      }
    },
  };
}

// Общая пружинка лент-каруселей лендинга — канон секции «Управляйте
// арендой» (sections/rentals-carousel.tsx), её используют и отзывы
// (sections/testimonials-carousel.tsx): глайд с мягким разгоном-торможением,
// лёгким перелётом цели и упругим возвратом, докатка после свайпа по
// простою скролла. Позиции покоя (центр окна, шаг ленты, TOUCH_INSET)
// считает вызывающая сторона — модуль ведёт только анимацию и её таймеры.
// Ленту и reduced-motion фабрика берёт через функции-доступители и читает
// их в момент анимации: сами ленты — React-компоненты, а чтение их ref-ов
// допустимо только вне рендера.

// Глайд: длительность растёт с дистанцией, потолок 850мс.
const GLIDE_BASE_MS = 350;
const GLIDE_MAX_MS = 850;
const GLIDE_MS_PER_PX = 0.25;

// Пружинка: лёгкий перелёт цели и упругий возврат.
const OVERSHOOT_MIN_PX = 6;
const OVERSHOOT_MAX_PX = 24;
const OVERSHOOT_RATIO = 0.05;
const OVERSHOOT_SETTLE_MS = 360;
const HOP_MAX_PX = 40; // короче — мягкий переход без перелёта
const HOP_MS = 420;
// Докатка после свайпа: простой скролла, прежде сами тянем к ближайшей.
const SETTLE_IDLE_MS = 140;

// Ускорения глайда: разгон-торможение основного хода и мягкая досадка
// после перелёта цели.
const easeInOutCubic = (t: number) =>
  t < 0.5 ? 4 * t * t * t : 1 - (-2 * t + 2) ** 3 / 2;
const easeOutCubic = (t: number) => 1 - (1 - t) ** 3;

// Планировщик докатки: откладывает settle до простоя скролла.
type ScheduleSettle = (settle: () => void) => void;

export type SpringOptions = {
  // Лента — relative-контейнер: scrollLeft построен на offsetLeft карточек.
  scroller: () => HTMLDivElement | null;
  // Актуальное состояние prefers-reduced-motion (может поменяться на лету).
  reduced: () => boolean;
  // Посадка пружинки: досчитать после возврата (посадочная докрутка отзывов
  // на десктопе). Таймер докатки живёт внутри пружинки, поэтому планировщик
  // отдаётся аргументом.
  onLand?: (scheduleSettle: ScheduleSettle) => void;
};

export type Spring = {
  // Пружинка к позиции покоя targetLeft; под reduced-motion — мгновенно.
  glideTo: (targetLeft: number) => void;
  // Ручной обрыв пружинки (pointerdown/wheel на ленте).
  stopGlide: () => void;
  // Идёт ли анимация: скролл-события во время неё докатку не порождают.
  isGliding: () => boolean;
  scheduleSettle: ScheduleSettle;
  // Снять таймеры пружинки при размонтировании ленты.
  destroy: () => void;
};

export function createSpring({
  scroller,
  reduced,
  onLand,
}: SpringOptions): Spring {
  // Состояние анимации — мутируемые боксы в замыкании (как useRef, только
  // живут у пружинки, а не у компонента): кадр, флаг полёта, таймер докатки.
  const glideRaf = { current: 0 };
  const glideActive = { current: false };
  const settleTimer = { current: null as ReturnType<typeof setTimeout> | null };

  const stopGlide = () => {
    cancelAnimationFrame(glideRaf.current);
    glideActive.current = false;
  };

  // Докатка по простою: перезапускаем таймер с нулевым отсчётом. Пока летит
  // пружинка, скролл-события докатку не планируют — после посадки линию
  // перепроверит сама (onLand).
  const scheduleSettle: ScheduleSettle = (settle) => {
    if (glideActive.current) {
      return;
    }
    if (settleTimer.current) {
      clearTimeout(settleTimer.current);
    }
    settleTimer.current = setTimeout(settle, SETTLE_IDLE_MS);
  };

  // Пружинка: мягкий разгон-торможение, лёгкий перелёт цели, упругий возврат.
  // Короткие ходы (< HOP_MAX_PX) — просто плавный переход без перелёта.
  const glideTo = (targetLeft: number) => {
    const el = scroller();
    if (!el) {
      return;
    }
    cancelAnimationFrame(glideRaf.current);
    if (settleTimer.current) {
      clearTimeout(settleTimer.current);
      settleTimer.current = null;
    }
    const left = Math.max(
      0,
      Math.min(targetLeft, el.scrollWidth - el.clientWidth),
    );
    glideActive.current = true;
    const finish = () => {
      glideActive.current = false;
      onLand?.(scheduleSettle);
    };
    const from = el.scrollLeft;
    const delta = left - from;
    if (reduced() || Math.abs(delta) < 1) {
      el.scrollLeft = left;
      finish();
      return;
    }
    const start = performance.now();
    if (Math.abs(delta) < HOP_MAX_PX) {
      const step = (now: number) => {
        const t = Math.min(1, (now - start) / HOP_MS);
        el.scrollLeft = from + delta * easeInOutCubic(t);
        if (t < 1) {
          glideRaf.current = requestAnimationFrame(step);
        } else {
          finish();
        }
      };
      glideRaf.current = requestAnimationFrame(step);
      return;
    }
    const peak =
      left +
      Math.sign(delta) *
        Math.min(
          OVERSHOOT_MAX_PX,
          Math.max(OVERSHOOT_MIN_PX, Math.abs(delta) * OVERSHOOT_RATIO),
        );
    const mainDuration = Math.min(
      GLIDE_MAX_MS,
      GLIDE_BASE_MS + Math.abs(delta) * GLIDE_MS_PER_PX,
    );
    const step = (now: number) => {
      const t1 = (now - start) / mainDuration;
      if (t1 < 1) {
        el.scrollLeft = from + (peak - from) * easeInOutCubic(Math.max(0, t1));
        glideRaf.current = requestAnimationFrame(step);
        return;
      }
      const t2 = (now - start - mainDuration) / OVERSHOOT_SETTLE_MS;
      el.scrollLeft =
        peak + (left - peak) * easeOutCubic(Math.min(1, Math.max(0, t2)));
      if (t2 < 1) {
        glideRaf.current = requestAnimationFrame(step);
      } else {
        el.scrollLeft = left;
        finish();
      }
    };
    glideRaf.current = requestAnimationFrame(step);
  };

  const destroy = () => {
    cancelAnimationFrame(glideRaf.current);
    if (settleTimer.current) {
      clearTimeout(settleTimer.current);
    }
  };

  const isGliding = () => glideActive.current;

  return { glideTo, stopGlide, isGliding, scheduleSettle, destroy };
}

// Индекс карточки, ближайшей к целевой точке (−1, если карточек ещё нет).
// Общий для активной точки/«полки» и для докатки после свайпа; позицию
// покоя найденной карточки считает вызывающая сторона.
export function nearestCardIndex(
  nodes: ArrayLike<HTMLElement | null>,
  target: number,
): number {
  let best = -1;
  let bestDist = Infinity;
  for (let i = 0; i < nodes.length; i += 1) {
    const card = nodes[i];
    if (!card) {
      continue;
    }
    const dist = Math.abs(card.offsetLeft + card.offsetWidth / 2 - target);
    if (dist < bestDist) {
      bestDist = dist;
      best = i;
    }
  }
  return best;
}

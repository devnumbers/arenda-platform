// Нормализация оси колеса для горизонтальных лент — по образцу
// Mousewheel-модуля Swiper (modules/mousewheel.mjs, normalize()):
// `if (ev.shiftKey && !pX) { pX = pY; pY = 0; }`. Браузеры делятся:
// Windows-браузеры сами ремапят Shift+вертикальное колесо в deltaX,
// macOS-браузеры присылают вертикальную дельту с голым shiftKey —
// без переноса Shift+колесо у таких браузеров не шагает ленту
// (гейт оси отсекает событие, поймано на живой приёмке 27.10).
export function wheelAxisOf(event: WheelEvent): { x: number; y: number } {
  let { deltaX: x, deltaY: y } = event;
  if (event.shiftKey && !x) {
    x = y;
    y = 0;
  }
  return { x, y };
}

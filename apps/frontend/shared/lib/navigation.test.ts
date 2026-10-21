import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  type AppRouter,
  goBack,
  pushPage,
  registerPageTransitionHandler,
  replacePage,
} from './navigation';

/** Хелперы навигации с каналом шторки переходов: с зарегистрированным
 * обработчиком (оверлей в ScreenLayout) переход уходит под лист —
 * обработчик сам решит, когда его выполнить; без оверлея (логин,
 * лендинг) — прямой переход, как раньше. goBack под шторкой тоже:
 * history.back()/replace-фолбэк едут внутри perform. */

function createRouter(): AppRouter {
  return { push: vi.fn(), replace: vi.fn() } as unknown as AppRouter;
}

function performOnce(handler: ReturnType<typeof vi.fn>): void {
  const perform = handler.mock.calls[0]?.[0] as () => void;
  perform();
}

beforeEach(() => {
  registerPageTransitionHandler(null);
});

describe('pushPage / replacePage: канал шторки', () => {
  it('с обработчиком переход уходит в него, а не в роутер напрямую', () => {
    const router = createRouter();
    const handler = vi.fn();
    registerPageTransitionHandler(handler);

    pushPage(router, '/operations');

    expect(router.push).not.toHaveBeenCalled();
    expect(handler).toHaveBeenCalledTimes(1);

    performOnce(handler);
    expect(router.push).toHaveBeenCalledWith('/operations');
  });

  it('replacePage кладёт в perform router.replace', () => {
    const router = createRouter();
    const handler = vi.fn();
    registerPageTransitionHandler(handler);

    replacePage(router, '/operations');

    performOnce(handler);
    expect(router.replace).toHaveBeenCalledWith('/operations');
    expect(router.push).not.toHaveBeenCalled();
  });

  it('без обработчика переход прямой — шторки нет (логин, лендинг)', () => {
    const router = createRouter();

    pushPage(router, '/operations');
    replacePage(router, '/payments');

    expect(router.push).toHaveBeenCalledWith('/operations');
    expect(router.replace).toHaveBeenCalledWith('/payments');
  });

  it('снятие обработчика возвращает прямой переход', () => {
    const router = createRouter();
    registerPageTransitionHandler(vi.fn());
    registerPageTransitionHandler(null);

    pushPage(router, '/operations');

    expect(router.push).toHaveBeenCalledWith('/operations');
  });
});

describe('goBack: возврат под шторкой', () => {
  it('с обработчиком history.back() уходит в perform', () => {
    const router = createRouter();
    const handler = vi.fn();
    registerPageTransitionHandler(handler);
    const historyBack = vi.fn();
    vi.stubGlobal('window', { history: { length: 3, back: historyBack } });

    goBack(router, '/properties');

    expect(historyBack).not.toHaveBeenCalled();
    performOnce(handler);

    expect(historyBack).toHaveBeenCalledTimes(1);
    expect(router.replace).not.toHaveBeenCalled();
    vi.unstubAllGlobals();
  });

  it('без истории — replace-фолбэк внутри perform', () => {
    const router = createRouter();
    const handler = vi.fn();
    registerPageTransitionHandler(handler);
    vi.stubGlobal('window', { history: { length: 1, back: vi.fn() } });

    goBack(router, '/properties');

    performOnce(handler);

    expect(router.replace).toHaveBeenCalledWith('/properties');
    vi.unstubAllGlobals();
  });

  it('без обработчика — прежнее поведение: сразу back или replace', () => {
    const router = createRouter();
    const historyBack = vi.fn();
    vi.stubGlobal('window', { history: { length: 3, back: historyBack } });

    goBack(router, '/properties');
    expect(historyBack).toHaveBeenCalledTimes(1);

    vi.stubGlobal('window', { history: { length: 1, back: vi.fn() } });
    goBack(router, '/properties');
    expect(router.replace).toHaveBeenCalledWith('/properties');
    vi.unstubAllGlobals();
  });
});

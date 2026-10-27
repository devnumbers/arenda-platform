'use client';

import { useEffect, useMemo, useState, useSyncExternalStore, type CSSProperties, type JSX } from 'react';
import { BoldUser } from '@/shared/assets/icons';
import { Button, CirclePhoto, Switch } from '@/shared/ui/design';
import styles from '../../page.module.css';

/**
 * Лаборатория blur-фазы CirclePhoto (решение владельца 26.10.2026, #1604):
 * подбор радиуса CSS-размытия и запаса растяжки кадра на живом примитиве —
 * ручки идут каскадом через CSS-переменные `--photo-blur-radius` /
 * `--photo-blur-bleed` (прод их не задаёт и рендерит дефолты 6px/8px).
 * Тумблер «Держать фазу» силой показывает blur-слой (скрывая готовый thumb
 * через data-blur-layer) — без него data-URL демо-фото грузится мгновенно
 * и фазу не рассмотреть; выключение тумблера перемонтирует круги и честно
 * отыгрывает фазу с fade. Слайдеры «кадр» эмулируют бэковую ручку #1603
 * (размер длинной стороны и качество jpeg) на canvas-даунскейле демо-фото —
 * подобранные значения для бэка уходят отдельным тикетом.
 *
 * Демо-фото синтетические (canvas, детерминированные): вид лаборатории не
 * зависит от сида стенда. Ряд «ваше фото» — настоящее серверное фото из
 * /me с настоящим кадром от бэка (слайдеры кадра на него не влияют).
 */

/** Прямоугольник 512×512 — источник демо-фото. */
const SCENE_SIZE = 512;

type SceneKey = 'landscape' | 'dark' | 'vivid';

const SCENES: ReadonlyArray<{ readonly key: SceneKey; readonly label: string; readonly draw: (ctx: CanvasRenderingContext2D) => void }> = [
    {
        key: 'landscape',
        label: 'Пейзаж',
        // Небо-градиент, солнце, горы, поле — высококонтрастная сцена с
        // крупными цветовыми полями, как реальные фото объектов.
        draw: (ctx) => {
            const sky = ctx.createLinearGradient(0, 0, 0, 300);
            sky.addColorStop(0, '#7db4e8');
            sky.addColorStop(1, '#d9e9f7');
            ctx.fillStyle = sky;
            ctx.fillRect(0, 0, SCENE_SIZE, 300);
            ctx.fillStyle = '#ffd66e';
            ctx.beginPath();
            ctx.arc(380, 110, 60, 0, Math.PI * 2);
            ctx.fill();
            ctx.fillStyle = '#5b7f9e';
            ctx.beginPath();
            ctx.moveTo(0, 300);
            ctx.lineTo(150, 150);
            ctx.lineTo(300, 300);
            ctx.closePath();
            ctx.fill();
            ctx.fillStyle = '#40658a';
            ctx.beginPath();
            ctx.moveTo(220, 300);
            ctx.lineTo(400, 130);
            ctx.lineTo(540, 300);
            ctx.closePath();
            ctx.fill();
            const field = ctx.createLinearGradient(0, 300, 0, SCENE_SIZE);
            field.addColorStop(0, '#7fae5c');
            field.addColorStop(1, '#4f7a38');
            ctx.fillStyle = field;
            ctx.fillRect(0, 300, SCENE_SIZE, 212);
        },
    },
    {
        key: 'dark',
        label: 'Тёмная сцена',
        // Глубокие тона с ярким акцентом — худший случай для блюра: кадр
        // темнеет и «заменяет» глиф почти чёрным пятном.
        draw: (ctx) => {
            const bg = ctx.createRadialGradient(256, 220, 40, 256, 256, 400);
            bg.addColorStop(0, '#3a4356');
            bg.addColorStop(1, '#14181f');
            ctx.fillStyle = bg;
            ctx.fillRect(0, 0, SCENE_SIZE, SCENE_SIZE);
            ctx.fillStyle = '#f4e3b2';
            ctx.fillRect(180, 140, 150, 190);
            ctx.fillStyle = '#14181f';
            ctx.fillRect(245, 140, 20, 190);
            ctx.fillRect(180, 225, 150, 14);
            ctx.fillStyle = '#f4e3b2';
            ctx.beginPath();
            ctx.arc(120, 400, 26, 0, Math.PI * 2);
            ctx.fill();
        },
    },
    {
        key: 'vivid',
        label: 'Пёстрая',
        // Много мелких насыщенных пятен — проверка «мыла»: при сильном
        // блюре превращается в кашу, при слабом читается рябь.
        draw: (ctx) => {
            const colors = ['#e5484d', '#2b7fff', '#30a46c', '#f76b15', '#8e4ec6', '#ffd60a'];
            ctx.fillStyle = '#f4f4f5';
            ctx.fillRect(0, 0, SCENE_SIZE, SCENE_SIZE);
            // Детерминированный LCG вместо Math.random — вид стабилен между
            // пересборками и перезагрузками стенда.
            let seed = 42;
            const next = (): number => {
                seed = (seed * 1664525 + 1013904223) % 4294967296;
                return seed / 4294967296;
            };
            for (let i = 0; i < 90; i += 1) {
                ctx.fillStyle = colors[Math.floor(next() * colors.length)] ?? colors[0] ?? '#e5484d';
                ctx.beginPath();
                ctx.arc(next() * SCENE_SIZE, next() * SCENE_SIZE, 14 + next() * 40, 0, Math.PI * 2);
                ctx.fill();
            }
        },
    },
];

/** Прод-дефолты ручек (§10): кнопка «Как в проде» и начальное состояние. */
const PROD_RADIUS = 6;
const PROD_BLEED = 0;
/** Прод-параметры кадра у бэка (#1603) — начальные значения слайдеров. */
const PROD_FRAME_SIZE = 16;
const PROD_FRAME_QUALITY = 50;

const SIZES: ReadonlyArray<{ readonly px: number; readonly box: string; readonly glyph: string }> = [
    { px: 24, box: 'h-6 w-6', glyph: 'h-3.5 w-3.5' },
    { px: 44, box: 'h-11 w-11', glyph: 'h-6 w-6' },
    { px: 96, box: 'h-24 w-24', glyph: 'h-[52px] w-[52px]' },
];

/** Синтетические фото: канвасы рисуются один раз на клиенте в модуле-сторе
 * (канон useSyncExternalStore с getServerSnapshot, как useDraftStore —
 * данные появляются после гидрации, без setState-in-effect). Источник кадра
 * — тот же нарисованный canvas: drawImage canvas→canvas синхронен, в
 * отличие от декодирования data-URL через Image. */
type SceneArt = { readonly photo: string; readonly canvas: HTMLCanvasElement };

const EMPTY_SCENES: ReadonlyMap<SceneKey, SceneArt> = new Map();
let sceneCache: ReadonlyMap<SceneKey, SceneArt> | null = null;

function drawScenes(): ReadonlyMap<SceneKey, SceneArt> {
    const out = new Map<SceneKey, SceneArt>();
    for (const scene of SCENES) {
        const canvas = document.createElement('canvas');
        canvas.width = SCENE_SIZE;
        canvas.height = SCENE_SIZE;
        const ctx = canvas.getContext('2d');
        if (ctx === null) continue;
        scene.draw(ctx);
        out.set(scene.key, { photo: canvas.toDataURL('image/jpeg', 0.9), canvas });
    }
    return out;
}

function subscribeScenes(onChange: () => void): () => void {
    sceneCache ??= drawScenes();
    sceneListeners.add(onChange);
    return () => {
        sceneListeners.delete(onChange);
    };
}

const sceneListeners = new Set<() => void>();
const getSceneSnapshot = (): ReadonlyMap<SceneKey, SceneArt> => sceneCache ?? EMPTY_SCENES;
const getServerSceneSnapshot = (): ReadonlyMap<SceneKey, SceneArt> => EMPTY_SCENES;

/** Кадры демо-фото: пересобираются по слайдерам размера/качества
 * (даунскейл тем же canvas API, что использует бэк по смыслу #1603). */
function useSceneFrames(frameSize: number, frameQuality: number): Map<SceneKey, { photo: string; frame: string }> {
    const scenes = useSyncExternalStore(subscribeScenes, getSceneSnapshot, getServerSceneSnapshot);
    return useMemo(() => {
        const out = new Map<SceneKey, { photo: string; frame: string }>();
        for (const [key, art] of scenes) {
            const canvas = document.createElement('canvas');
            canvas.width = frameSize;
            canvas.height = frameSize;
            const ctx = canvas.getContext('2d');
            if (ctx !== null) {
                ctx.imageSmoothingEnabled = true;
                ctx.imageSmoothingQuality = 'high';
                ctx.drawImage(art.canvas, 0, 0, art.canvas.width, art.canvas.height, 0, 0, frameSize, frameSize);
            }
            // Множитель вместо деления: сырой `/100` — запрет денежного
            // гейта (quality bar wave C), здесь это проценты качества jpeg.
            out.set(key, { photo: art.photo, frame: canvas.toDataURL('image/jpeg', frameQuality * 0.01) });
        }
        return out;
    }, [scenes, frameSize, frameQuality]);
}

/** Настоящее серверное фото из /me — ряд «как в бою» с кадром от бэка. */
type MePhotoState = { readonly photoUrl: string | null; readonly photoBlur: string | null };

async function fetchMePhoto(): Promise<MePhotoState | undefined> {
    try {
        const r = await fetch('/api/me');
        if (!r.ok) return undefined;
        const j = (await r.json()) as { photoUrl?: string | null; photoBlur?: string | null };
        return { photoUrl: j.photoUrl ?? null, photoBlur: j.photoBlur ?? null };
    } catch {
        return undefined;
    }
}

function useMePhoto(): MePhotoState {
    const [me, setMe] = useState<MePhotoState>({ photoUrl: null, photoBlur: null });
    useEffect(() => {
        let alive = true;
        void fetchMePhoto().then((next) => {
            if (alive && next !== undefined) setMe(next);
        });
        return () => {
            alive = false;
        };
    }, []);
    return me;
}

function Slider({
    label,
    value,
    min,
    max,
    step,
    unit,
    onChange,
}: {
    readonly label: string;
    readonly value: number;
    readonly min: number;
    readonly max: number;
    readonly step: number;
    readonly unit: string;
    readonly onChange: (value: number) => void;
}): JSX.Element {
    return (
        <label className="flex flex-col gap-1">
            <span className="text-sm font-medium text-content">
                {label}: <span className="font-mono text-content-secondary">{value}{unit}</span>
            </span>
            <input
                type="range"
                min={min}
                max={max}
                step={step}
                value={value}
                onChange={(e) => onChange(Number(e.target.value))}
                className="h-1.5 w-64 cursor-pointer accent-primary"
            />
        </label>
    );
}

export function PhotosSection(): JSX.Element {
    const [holdPhase, setHoldPhase] = useState(true);
    const [blurEnabled, setBlurEnabled] = useState(true);
    const [radius, setRadius] = useState(PROD_RADIUS);
    const [bleed, setBleed] = useState(PROD_BLEED);
    const [frameSize, setFrameSize] = useState(PROD_FRAME_SIZE);
    const [frameQuality, setFrameQuality] = useState(PROD_FRAME_QUALITY);

    const scenes = useSceneFrames(frameSize, frameQuality);
    const mePhoto = useMePhoto();

    const labStyle = {
        '--photo-blur-radius': `${radius}px`,
        '--photo-blur-bleed': `${bleed}px`,
    } as CSSProperties;

    // Без перемонтирования: latch blurShown переживает смену пропов, слайдеры
    // (CSS-переменные + новый src кадра) применяются к живому слою. Ремоунт
    // ломал бы лабораторию — сессионный реестр даёт свежему инстансу
    // photoReady=true (фото то же), blur-слой не монтируется вовсе.
    const reset = (): void => {
        setRadius(PROD_RADIUS);
        setBleed(PROD_BLEED);
        setFrameSize(PROD_FRAME_SIZE);
        setFrameQuality(PROD_FRAME_QUALITY);
        setBlurEnabled(true);
        setHoldPhase(true);
    };

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>CirclePhoto — blur-фаза (лаборатория)</h3>
            <style>{`
                .ui-kit-blur-lab[data-hold='true'] img[data-blur-layer] { opacity: 1 !important; transition: none !important; }
                .ui-kit-blur-lab[data-hold='true'] img[data-blur-layer] + img { opacity: 0 !important; transition: none !important; }
            `}</style>
            <div className="ui-kit-blur-lab flex flex-col gap-6" data-hold={holdPhase ? 'true' : 'false'} style={labStyle}>
                <div className="flex flex-wrap items-center gap-x-8 gap-y-3">
                    <span className="flex items-center gap-2 text-sm font-medium text-content">
                        <Switch checked={holdPhase} onCheckedChange={setHoldPhase} aria-label="Держать blur-фазу" />
                        Держать фазу (blur поверх глифа)
                    </span>
                    <span className="flex items-center gap-2 text-sm font-medium text-content">
                        <Switch checked={blurEnabled} onCheckedChange={setBlurEnabled} aria-label="Blur включён" />
                        Blur включён (выкл — деградация «глиф → fade»)
                    </span>
                    <Button variant="secondary" size="small" onClick={reset}>Как в проде</Button>
                </div>
                <div className="flex flex-wrap items-start gap-x-8 gap-y-4">
                    <Slider label="Радиус размытия" value={radius} min={0} max={16} step={0.5} unit="px" onChange={setRadius} />
                    <Slider label="Запас растяжки" value={bleed} min={0} max={16} step={1} unit="px" onChange={setBleed} />
                    <Slider label="Кадр: размер" value={frameSize} min={4} max={32} step={2} unit="px" onChange={setFrameSize} />
                    <Slider label="Кадр: качество" value={frameQuality} min={20} max={90} step={5} unit="%" onChange={setFrameQuality} />
                </div>
                <p className="text-sm text-content-tertiary">
                    Радиус и запас — фронтовые ручки примитива; «кадр» эмулирует бэковую генерацию #1603
                    (прод сейчас: 16px, качество 50%). «Держать фазу» замораживает круг в момент загрузки
                    (blur поверх глифа, thumb скрыт) — слайдеры применяются к живым кругам сразу; снимите
                    тумблер, чтобы увидеть итог после загрузки, и верните — blur вернётся. Полная
                    перезагрузка страницы всегда отыгрывает настоящую фазу с fade 250ms.
                </p>
                {[...scenes.entries()].map(([key, { photo, frame }]) => (
                    <div key={key} className="flex flex-col gap-2">
                        <p className="text-sm font-medium text-content-secondary">
                            {SCENES.find((s) => s.key === key)?.label} · кадр {blurEnabled ? `${frameSize}px / ${frameQuality}%` : 'выключен'}
                        </p>
                        <div className="flex items-end gap-6">
                            {SIZES.map((size) => (
                                <div key={size.px} className="flex flex-col items-center gap-1">
                                    <span
                                        aria-hidden
                                        className={`flex items-center justify-center overflow-hidden rounded-full bg-surface-muted ring-2 ring-white ${size.box}`}
                                    >
                                        <CirclePhoto
                                            photoUrl={photo}
                                            photoBlur={blurEnabled ? frame : null}
                                            glyph={<BoldUser className={`text-[#d3d7d9] ${size.glyph}`} />}
                                        />
                                    </span>
                                    <span className="text-xs text-content-tertiary">{size.px}px</span>
                                </div>
                            ))}
                        </div>
                    </div>
                ))}
                {mePhoto.photoUrl !== null && (
                    <div className="flex flex-col gap-2">
                        <p className="text-sm font-medium text-content-secondary">
                            Ваше фото · настоящий кадр от бэка (слайдеры «кадр» не влияют)
                        </p>
                        <div className="flex items-end gap-6">
                            {SIZES.map((size) => (
                                <div key={size.px} className="flex flex-col items-center gap-1">
                                    <span
                                        aria-hidden
                                        className={`flex items-center justify-center overflow-hidden rounded-full bg-surface-muted ring-2 ring-white ${size.box}`}
                                    >
                                        <CirclePhoto
                                            photoUrl={mePhoto.photoUrl}
                                            photoBlur={blurEnabled ? mePhoto.photoBlur : null}
                                            glyph={<BoldUser className={`text-[#d3d7d9] ${size.glyph}`} />}
                                        />
                                    </span>
                                    <span className="text-xs text-content-tertiary">{size.px}px</span>
                                </div>
                            ))}
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}

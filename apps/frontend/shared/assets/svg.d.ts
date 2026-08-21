// Ambient declaration for SVG imports (quality bar wave A, bar #330):
// @svgr/webpack (next.config.ts) compiles every `*.svg` import into a React
// component. Next's image-types fallback types these modules as `any`, which
// is the sole source of the `no-unsafe-*` findings under type-checked linting
// (wave B) — this declaration gives the imports their real shape.
declare module '*.svg' {
    import type { FC, SVGProps } from 'react';

    const ReactComponent: FC<SVGProps<SVGSVGElement>>;
    export default ReactComponent;
}

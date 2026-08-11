import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: 'standalone',
  reactCompiler: true,
  headers: async () => [
    {
      // The service worker script must always be fetched fresh so updates are
      // picked up promptly (byte-compare update check). `public/` is served
      // with max-age=0,must-revalidate by default; this reinforces no-store
      // specifically for /sw.js.
      source: '/sw.js',
      headers: [
        { key: 'Cache-Control', value: 'no-store' },
      ],
    },
  ],
  turbopack: {
    rules: {
      '*.svg': {
        loaders: [
          {
            loader: '@svgr/webpack',
            options: {
              svgoConfig: {
                plugins: [
                  {
                    name: 'preset-default',
                    params: {
                      overrides: {
                        removeViewBox: false,
                      },
                    },
                  },
                  'prefixIds',
                ],
              },
            },
          },
        ],
        as: '*.js',
      },
    },
  },
};

export default nextConfig;

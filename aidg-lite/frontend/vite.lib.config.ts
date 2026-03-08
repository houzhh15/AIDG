import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';
import dts from 'vite-plugin-dts';

/**
 * Library build configuration.
 * Run: npm run build:lib
 *
 * Output:
 *   dist-lib/aidg-lite-ui.es.js   — ES module
 *   dist-lib/aidg-lite-ui.cjs.js  — CommonJS
 *   dist-lib/index.d.ts            — Type declarations
 */
export default defineConfig({
  plugins: [
    react(),
    dts({
      include: ['src/lib/**', 'src/api/**', 'src/components/**', 'src/hooks/**',
                'src/contexts/**', 'src/services/**', 'src/types/**',
                'src/utils/**', 'src/config/**', 'src/constants/**'],
      outDir: 'dist-lib',
      insertTypesEntry: true,
      // rollupTypes requires api-extractor which doesn't support 'export * as'
      rollupTypes: false,
    }),
  ],

  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },

  build: {
    outDir: 'dist-lib',
    lib: {
      entry: path.resolve(__dirname, 'src/lib/index.ts'),
      name: 'AidgLiteUi',
      fileName: (format) => `aidg-lite-ui.${format}.js`,
      formats: ['es', 'cjs'],
    },
    rollupOptions: {
      // Packages the consumer must supply — do NOT bundle them.
      external: [
        'react',
        'react-dom',
        'react/jsx-runtime',
        'antd',
        'axios',
        'mermaid',
        '@monaco-editor/react',
        'react-markdown',
        'react-syntax-highlighter',
        'rehype-katex',
        'rehype-raw',
        'remark-gfm',
        'remark-math',
        'katex',
      ],
      output: {
        globals: {
          react: 'React',
          'react-dom': 'ReactDOM',
          antd: 'antd',
          axios: 'axios',
        },
        // Inject CSS into the output for ES builds
        assetFileNames: 'aidg-lite-ui.[ext]',
      },
    },
    sourcemap: true,
    emptyOutDir: true,
  },
});

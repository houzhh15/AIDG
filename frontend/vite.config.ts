import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig(({ mode }) => {
  // Load env file based on `mode` in the current working directory
  const env = loadEnv(mode, process.cwd(), '');
  
  return {
    plugins: [
      react(),
      // Custom plugin to fix modulepreload order - React must load first
      {
        name: 'fix-modulepreload-order',
        apply: 'build',
        transformIndexHtml(html) {
          // Extract all modulepreload links
          const preloadRegex = /<link rel="modulepreload"[^>]*>/g;
          const preloads = html.match(preloadRegex) || [];
          
          // Sort preloads: react-vendor first, then others
          const sortedPreloads = preloads.sort((a, b) => {
            const aIsReact = a.includes('react-vendor');
            const bIsReact = b.includes('react-vendor');
            if (aIsReact && !bIsReact) return -1; // react-vendor comes first
            if (!aIsReact && bIsReact) return 1;
            return 0; // Keep original order for others
          });
          
          // Remove all existing modulepreload links
          let result = html.replace(preloadRegex, '');
          
          // Insert sorted preloads back (after the main script tag)
          const scriptTagRegex = /<script type="module"[^>]*><\/script>/;
          result = result.replace(scriptTagRegex, (match) => {
            return match + '\n    ' + sortedPreloads.join('\n    ');
          });
          
          return result;
        },
      },
    ],
    
    // Path resolution
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    
    // Development server configuration
    server: {
      port: 5173,
      host: true, // Listen on all addresses for Docker support
      proxy: {
        '/api': {
          target: env.VITE_API_BASE_URL || 'http://localhost:8000',
          changeOrigin: true,
          ws: true,
        },
      },
    },
    
    // Build configuration
    build: {
      outDir: 'dist',
      sourcemap: mode === 'development', // Only generate sourcemap in dev
      // Disable code splitting to fix module loading order issues
      rollupOptions: {
        output: {
          manualChunks: undefined, // Disable all code splitting
        },
      },
      // Build performance
      chunkSizeWarningLimit: 1000, // KB
      minify: 'terser',
      terserOptions: {
        compress: {
          drop_console: mode === 'production',
          drop_debugger: mode === 'production',
          pure_funcs: mode === 'production' ? ['console.log', 'console.debug'] : [],
        },
        format: {
          comments: false, // Remove comments in production
        },
      },
      // Asset optimization
      assetsInlineLimit: 4096, // 4KB - inline small assets as base64
      cssCodeSplit: true, // Split CSS by chunk
    },
    
    // Define global constants
    define: {
      __APP_VERSION__: JSON.stringify(process.env.npm_package_version || '1.0.0'),
      __BUILD_TIME__: JSON.stringify(new Date().toISOString()),
    },
  };
});

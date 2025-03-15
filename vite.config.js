import { defineConfig } from 'vite';
import { resolve } from 'path';
import terser from '@rollup/plugin-terser';
import autoprefixer from 'autoprefixer';

export default defineConfig({
  root: './src',
  base: '/assets/',
  build: {
    outDir: '../assets',
    emptyOutDir: true,
    sourcemap: false,
    rollupOptions: {
      input: {
        main: resolve(__dirname, 'src/main.js'),
        cycling: resolve(__dirname, 'src/cycling.js'),
      },
      output: {
        entryFileNames: '[name].min.js',
        assetFileNames: assetInfo => {
          if (assetInfo.names && assetInfo.names[0].endsWith('.css')) {
            return '[name].min.css';
          }
          return '[name].[ext]';
        }
      },
      external: [],
    },
    minify: 'terser',
    terserOptions: {
      compress: {
        evaluate: false,
        drop_console: false,
        drop_debugger: true
      },
      format: {
        comments: false
      }
    }
  },
  css: {
    preprocessorOptions: {
      scss: {}
    },
    postcss: {
      plugins: [
        autoprefixer(),
      ]
    }
  },
  resolve: {
    alias: {
      'iDisqus.css': resolve(__dirname, './node_modules/disqus-php-api/dist/iDisqus.min.css'),
    }
  }
})
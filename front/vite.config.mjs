import path from 'node:path';
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import vuetify from 'vite-plugin-vuetify';
import checker from 'vite-plugin-checker';
import wasm from 'vite-plugin-wasm';

// https://vitejs.dev/config/
export default defineConfig({
    plugins: [
        vue(),
        vuetify(),
        checker({
            vueTsc: true,
        }),
        wasm(),
    ],
    server: {
        port: 8031,
    },
    resolve: {
        alias: {
            '@': path.resolve(__dirname, './src'),
        },
    },
    optimizeDeps: {
        exclude: [
            "brotli-dec-wasm",
        ],
    },
});

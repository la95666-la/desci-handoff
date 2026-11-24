import process from 'node:process';

import { defineConfig } from '@vben/vite-config';

import { loadEnv } from 'vite';

export default defineConfig(async (configEnv) => {
  const env = loadEnv(configEnv.mode, process.cwd(), '');
  const target = env.VITE_JOINCARE_API_TARGET || 'http://172.18.0.4:13004';
  const localFairLaunchTagged = Object.entries({ ...env, ...process.env }).some(
    ([key, value]) =>
      /^VITE_FAIR_LAUNCH_LOCAL_/i.test(key) &&
      String(value || '').trim() !== '',
  );
  if (configEnv.mode === 'production' && localFairLaunchTagged) {
    throw new Error(
      'Fair Launch LOCAL-ONLY authority is forbidden in production Admin builds',
    );
  }

  return {
    application: {},
    vite: {
      server: {
        // WSL2 + Windows NTFS: inotify 跨文件系统失效，HMR 需 polling
        watch: {
          usePolling: true,
          interval: 300,
        },
        proxy: {
          // 前端代码统一写 /api/v1/login，被代理改写到 /admin/v1/login
          '/api': {
            changeOrigin: true,
            rewrite: (path) => path.replace(/^\/api/, '/admin'),
            target,
            ws: false,
          },
        },
      },
    },
  };
});

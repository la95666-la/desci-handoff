import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { execSync } from 'node:child_process'
import { readFileSync } from 'node:fs'

// 环境配置映射（与 src/config/environments.js 保持一致）
const environments = {
  local: {
    // WSL2 docker DNAT 不可用，dev server 直接走 docker bridge IP；可由 .env.local 的
    // VITE_LOCAL_API_URL 覆盖。容器 IP 在 docker-compose 起 backend 后 `docker inspect jc-backend` 看
    apiBaseUrl: process.env.VITE_LOCAL_API_URL || 'http://172.18.0.4:13004'
  },
  test: {
    apiBaseUrl: 'http://apitest.joincare.network'
  },
  prod: {
    apiBaseUrl: 'https://api.joincare.network'  // 暂未配置
  }
}

const readGitSha = () => {
  try { return execSync('git rev-parse --short HEAD').toString().trim() } catch { return 'unknown' }
}
const readVersion = () => {
  try { return JSON.parse(readFileSync('./package.json', 'utf8')).version } catch { return '0.0.0' }
}

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  // 加载 .env 文件中的环境变量
  const env = loadEnv(mode, process.cwd(), '')
  const currentEnv = env.VITE_ENVIRONMENT || 'local'
  const envConfig = { ...(environments[currentEnv] || environments.local) }
  const mergedBuildEnv = { ...env, ...process.env }
  const localFairLaunchOptIn = String(mergedBuildEnv.VITE_FAIR_LAUNCH_LOCAL_WALLET_SUBMISSION || '').trim().toLowerCase() === 'true'
  const localFairLaunchTagged = Object.entries(mergedBuildEnv).some(([key, value]) =>
    (/^VITE_FAIR_LAUNCH_LOCAL_/i.test(key) || key === 'VITE_FAIR_LAUNCH_TEST_MODE') &&
    key !== 'VITE_FAIR_LAUNCH_LOCAL_WALLET_SUBMISSION' && String(value || '').trim() !== ''
  )
  if (mode === 'production' && (localFairLaunchOptIn || localFairLaunchTagged)) {
    throw new Error('Fair Launch local wallet submission or local manifest identity is forbidden in production builds')
  }
  const injectedApi = process.env.VITE_API_BASE_URL ?? env.VITE_API_BASE_URL

  if (injectedApi !== undefined) {
    envConfig.apiBaseUrl = injectedApi
  } else if (currentEnv !== 'local') {
    envConfig.apiBaseUrl = ''
  }

  console.log(`\n🚀 JoinCare Frontend — Environment: ${currentEnv}`)
  console.log(`   API Target: ${envConfig.apiBaseUrl || '(not configured)'}\n`)

  const buildTime = new Date().toISOString()
  const gitSha = readGitSha()
  const appVersion = readVersion()

  const config = {
    plugins: [vue()],
    // 编译时删除开发调试类 console；warn / error 保留（真正的问题依然可见）
    esbuild: {
      pure: mode === 'production' ? ['console.log', 'console.info', 'console.debug'] : [],
      drop:  mode === 'production' ? ['debugger'] : [],
    },
    define: {
      __APP_VERSION__: JSON.stringify(appVersion),
      __APP_GIT_SHA__: JSON.stringify(gitSha),
      __APP_BUILD_TIME__: JSON.stringify(buildTime),
      __APP_ENV_NAME__: JSON.stringify(currentEnv),
      __APP_API_BASE__: JSON.stringify(envConfig.apiBaseUrl || ''),
    },
    server: {
      // WSL2 + Windows 文件系统（/mnt/...）下，inotify 不会从 NTFS 透传到 WSL，
      // chokidar 用轮询才能感知 WSL 内写入的文件改动 → HMR 才能自动触发。
      watch: {
        usePolling: true,
        interval: 200,
      },
    },
  }

  // 仅在 local 环境下配置代理（test/prod 由前端直连后端 API）
  if (currentEnv === 'local' && envConfig.apiBaseUrl) {
    config.server.proxy = {
      '/client': {
        target: envConfig.apiBaseUrl,
        changeOrigin: true,
        configure: (proxy) => {
          // 请求发出时打印
          proxy.on('proxyReq', (proxyReq, req) => {
            const time = new Date().toLocaleTimeString()
            console.log(`\n[${time}] ➡️  ${req.method} ${req.url} → ${envConfig.apiBaseUrl}${req.url}`)

            // 打印请求体（POST/PUT）
            let bodyChunks = []
            req.on('data', (chunk) => bodyChunks.push(chunk))
            req.on('end', () => {
              if (bodyChunks.length > 0) {
                const body = Buffer.concat(bodyChunks).toString()
                console.log(`    📦 Body: ${body}`)
              }
            })
          })

          // 收到后端响应时打印
          proxy.on('proxyRes', (proxyRes, req) => {
            const time = new Date().toLocaleTimeString()
            let responseBody = ''
            proxyRes.on('data', (chunk) => { responseBody += chunk })
            proxyRes.on('end', () => {
              console.log(`[${time}] ⬅️  ${proxyRes.statusCode} ${req.method} ${req.url}`)
              try {
                const json = JSON.parse(responseBody)
                console.log(`    📄 Response:`, JSON.stringify(json, null, 2))
              } catch {
                console.log(`    📄 Response: ${responseBody.substring(0, 500)}`)
              }
            })
          })

          // 代理错误
          proxy.on('error', (err, req) => {
            const time = new Date().toLocaleTimeString()
            console.log(`[${time}] ❌ Proxy Error: ${req.method} ${req.url} — ${err.message}`)
          })
        }
      }
    }
  }

  return config
})

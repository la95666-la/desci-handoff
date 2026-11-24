#!/usr/bin/env node

/**
 * 环境切换脚本 (Node.js 版本)
 * 在 Windows、Mac、Linux 上都能用
 *
 * 用法:
 *   node scripts/switch-env.js local
 *   node scripts/switch-env.js test
 *   node scripts/switch-env.js prod
 */

import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

// 获取项目根目录（ESM 中没有 __filename / __dirname）
const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const projectRoot = path.dirname(__dirname)
const envFile = path.join(projectRoot, '.env.local')

// 环境配置
const environments = {
    local: {
        name: 'Local Dev',
        api: 'http://localhost:13004',
        ws: 'ws://localhost:13004'
    },
    test: {
        name: 'Testing',
        api: 'http://apitest.joincare.network',
        ws: 'ws://wstest.joincare.network'
    },
    prod: {
        name: 'Production',
        api: 'https://api.joincare.network',
        ws: 'wss://ws.joincare.network'
    }
}

// 获取命令行参数
const env = process.argv[2]

if (!env) {
    console.log('\nJoinCare Frontend Environment Switcher')
    console.log('========================================\n')
    console.log('Usage:')
    console.log('  node scripts/switch-env.js local  # Switch to local dev')
    console.log('  node scripts/switch-env.js test   # Switch to testing')
    console.log('  node scripts/switch-env.js prod   # Switch to production')
    console.log('')
    process.exit(1)
}

// 验证环境
if (!environments[env]) {
    console.error(`\nERROR: Unknown environment "${env}"`)
    console.error('Supported: local, test, prod\n')
    process.exit(1)
}

// 写入 .env.local 文件。切换环境会覆盖文件，从而机械清除任何本机 Fair Launch
// ephemeral manifest/opt-in；本机钱包测试必须由专用 harness 每次重新注入。
try {
    const config = environments[env]
    const content = `VITE_ENVIRONMENT=${env}\nVITE_FAIR_LAUNCH_TEST_MODE=disabled\nVITE_FAIR_LAUNCH_LOCAL_WALLET_SUBMISSION=false\n`

    fs.writeFileSync(envFile, content, 'utf8')

    console.log('\n✓ Environment switched successfully\n')
    console.log(`Environment: ${config.name}`)
    console.log(`API:         ${config.api}`)
    console.log(`WS:          ${config.ws}`)
    console.log('\nNext steps:')
    console.log('  1. npm run dev (development)')
    console.log('  2. npm run build (production build)\n')
} catch (error) {
    console.error(`\nERROR: Failed to write .env.local`)
    console.error(error.message, '\n')
    process.exit(1)
}

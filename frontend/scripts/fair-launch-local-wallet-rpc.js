#!/usr/bin/env node

import fs from 'node:fs'
import http from 'node:http'
import https from 'node:https'
import path from 'node:path'
import process from 'node:process'
import { Transaction } from 'ethers'

const LOOPBACK_HTTP = /^http:\/\/127\.0\.0\.1:([1-9]\d{0,4})$/
const RUN_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/
const MAX_BODY_BYTES = 2 * 1024 * 1024

function fail(message) {
  console.error(`fair-launch-local-wallet-rpc: ${message}`)
  process.exit(1)
}

const [targetUrl, certificatePath, privateKeyPath, authorityPath, runId, requestedPort = '0'] = process.argv.slice(2)
if (!targetUrl || !certificatePath || !privateKeyPath || !authorityPath || !runId) {
  fail('usage: node scripts/fair-launch-local-wallet-rpc.js <http://127.0.0.1:rpc-port> <server.crt> <server.key> <authority.json> <run-id> [loopback-port]')
}
if (!LOOPBACK_HTTP.test(targetUrl)) fail('target RPC must be exact http://127.0.0.1:<explicit-port>')
if (!RUN_ID.test(runId)) fail('run ID is invalid')
if (!/^(?:0|[1-9]\d{0,4})$/.test(requestedPort) || Number(requestedPort) > 65535) fail('wallet RPC port is invalid')

const target = new URL(targetUrl)
const tls = {
  cert: fs.readFileSync(certificatePath),
  key: fs.readFileSync(privateKeyPath),
  minVersion: 'TLSv1.2',
}

function responseHeaders(statusCode, contentType = 'application/json') {
  return {
    'access-control-allow-headers': 'content-type',
    'access-control-allow-methods': 'POST, OPTIONS',
    'access-control-allow-origin': '*',
    'cache-control': 'no-store',
    'content-type': contentType,
    'x-content-type-options': 'nosniff',
    'x-jc-fl-local-wallet-rpc': runId,
  }
}

function safeTransactionDiagnostic(rawBody) {
  try {
    const request = JSON.parse(rawBody.toString('utf8'))
    if (!request || typeof request !== 'object' || Array.isArray(request)) return null
    if (request.method === 'eth_sendRawTransaction') {
      const raw = Array.isArray(request.params) ? request.params[0] : null
      if (typeof raw !== 'string') return { method: request.method, invalidParams: true }
      const tx = Transaction.from(raw)
      return {
        method: request.method,
        hash: tx.hash,
        from: tx.from?.toLowerCase() || null,
        nonce: tx.nonce,
        chainId: tx.chainId?.toString() || null,
        to: tx.to?.toLowerCase() || null,
        selector: typeof tx.data === 'string' ? tx.data.slice(0, 10).toLowerCase() : null,
        value: tx.value?.toString() || '0',
        gasLimit: tx.gasLimit?.toString() || null,
        gasPrice: tx.gasPrice?.toString() || null,
        maxFeePerGas: tx.maxFeePerGas?.toString() || null,
        maxPriorityFeePerGas: tx.maxPriorityFeePerGas?.toString() || null,
      }
    }
    if (request.method === 'eth_sendTransaction') {
      const tx = Array.isArray(request.params) && request.params[0] && typeof request.params[0] === 'object'
        ? request.params[0]
        : {}
      return {
        method: request.method,
        from: String(tx.from || '').toLowerCase() || null,
        nonce: tx.nonce ?? null,
        chainId: tx.chainId ?? null,
        to: String(tx.to || '').toLowerCase() || null,
        selector: typeof tx.data === 'string' ? tx.data.slice(0, 10).toLowerCase() : null,
        value: tx.value ?? '0x0',
        gas: tx.gas ?? null,
        gasPrice: tx.gasPrice ?? null,
        maxFeePerGas: tx.maxFeePerGas ?? null,
        maxPriorityFeePerGas: tx.maxPriorityFeePerGas ?? null,
      }
    }
  } catch (error) {
    return { method: 'transaction_decode_failed', message: String(error?.message || error).slice(0, 200) }
  }
  return null
}

function safeUpstreamDiagnostic(rawBody) {
  try {
    const value = JSON.parse(rawBody.toString('utf8'))
    if (!value || typeof value !== 'object' || Array.isArray(value)) return { invalidResponse: true }
    if (value.error) {
      return {
        errorCode: value.error.code ?? null,
        errorMessage: String(value.error.message || '').slice(0, 300),
        errorDataMessage: String(value.error.data?.message || '').slice(0, 300) || null,
      }
    }
    return { resultHash: typeof value.result === 'string' && /^0x[0-9a-f]{64}$/i.test(value.result) ? value.result.toLowerCase() : null }
  } catch {
    return { invalidResponse: true }
  }
}

const server = https.createServer(tls, (request, response) => {
  if (request.method === 'OPTIONS') {
    response.writeHead(204, responseHeaders(204))
    response.end()
    return
  }
  if (request.method !== 'POST' || request.url !== '/') {
    response.writeHead(404, responseHeaders(404))
    response.end(JSON.stringify({ error: 'not_found' }))
    return
  }

  let bodyBytes = 0
  const chunks = []
  request.on('data', (chunk) => {
    bodyBytes += chunk.length
    if (bodyBytes > MAX_BODY_BYTES) request.destroy(new Error('request_too_large'))
    else chunks.push(chunk)
  })
  request.on('error', () => {
    if (!response.headersSent) response.writeHead(400, responseHeaders(400))
    response.end(JSON.stringify({ error: 'invalid_request' }))
  })
  request.on('end', () => {
    const requestBody = Buffer.concat(chunks)
    const transactionDiagnostic = safeTransactionDiagnostic(requestBody)
    if (transactionDiagnostic) {
      process.stderr.write(`[wallet-rpc-transaction] ${JSON.stringify({ runId, ...transactionDiagnostic })}\n`)
    }
    const upstream = http.request({
      hostname: target.hostname,
      port: Number(target.port),
      path: '/',
      method: 'POST',
      headers: {
        'content-type': request.headers['content-type'] || 'application/json',
        'content-length': bodyBytes,
      },
      timeout: 10_000,
    }, (upstreamResponse) => {
      const upstreamChunks = []
      upstreamResponse.on('data', (chunk) => upstreamChunks.push(chunk))
      upstreamResponse.on('end', () => {
        const upstreamBody = Buffer.concat(upstreamChunks)
        if (transactionDiagnostic) {
          process.stderr.write(`[wallet-rpc-result] ${JSON.stringify({
            runId,
            method: transactionDiagnostic.method,
            httpStatus: upstreamResponse.statusCode || 502,
            ...safeUpstreamDiagnostic(upstreamBody),
          })}\n`)
        }
        response.end(upstreamBody)
      })
      response.writeHead(upstreamResponse.statusCode || 502, responseHeaders(
        upstreamResponse.statusCode || 502,
        upstreamResponse.headers['content-type'] || 'application/json',
      ))
    })
    upstream.on('timeout', () => upstream.destroy(new Error('upstream_timeout')))
    upstream.on('error', () => {
      if (!response.headersSent) response.writeHead(502, responseHeaders(502))
      response.end(JSON.stringify({ error: 'upstream_unavailable' }))
    })
    upstream.end(requestBody)
  })
})

server.on('error', (error) => fail(error.message))
server.listen({ host: '127.0.0.1', port: Number(requestedPort), exclusive: true }, () => {
  const address = server.address()
  if (!address || typeof address === 'string') fail('failed to bind an exact loopback authority')
  const authority = {
    authority: 'TEST_ONLY_NOT_PRODUCTION_ADMITTED',
    runId,
    walletRpcUrl: `https://127.0.0.1:${address.port}`,
    canonicalRpcUrl: targetUrl,
    pid: process.pid,
    status: 'LOCAL_ONLY_WALLET_RPC_ADMITTED',
  }
  const output = `${JSON.stringify(authority)}\n`
  fs.mkdirSync(path.dirname(authorityPath), { recursive: true })
  fs.writeFileSync(authorityPath, output, { encoding: 'utf8', flag: 'wx', mode: 0o600 })
  process.stdout.write(output)
})

for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => server.close(() => process.exit(0)))
}

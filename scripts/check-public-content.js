#!/usr/bin/env node
const { execSync } = require('child_process')
const crypto = require('crypto')
const fs = require('fs')

const files = execSync('git ls-files', { encoding: 'utf8' })
  .trim()
  .split(/\r?\n/)
  .filter(Boolean)

const privateKeyMarker = ['PRIVATE', 'KEY'].join(' ')
const testVPSIP = ['107', '174', '123', '76'].join('\\.')
const blockedCredentialHashes = new Set([
  '78037ce405c5490cb4002b7bd05be5b004a54b93ccf7752e3035231d07c43ca1'
])

const secretChecks = [
  { name: 'test VPS IP', test: (content) => new RegExp(testVPSIP).test(content) },
  { name: 'test VPS credential', test: containsBlockedCredential },
  { name: 'private key', test: (content) => new RegExp(`BEGIN (OPENSSH|RSA|EC|DSA) ${privateKeyMarker}`).test(content) },
  { name: 'inline ssh private material', test: (content) => new RegExp(`OPENSSH ${privateKeyMarker}`).test(content) }
]

const blockedRuntimeFiles = [
  { name: 'runtime data directory', test: (file) => file === 'data' || file.startsWith('data/') },
  { name: 'runtime log directory', test: (file) => file === 'logs' || file.startsWith('logs/') || file.includes('/logs/') },
  { name: 'temporary work directory', test: (file) => file === '.tmp' || file.startsWith('.tmp/') },
  { name: 'build output directory', test: (file) => file === 'dist' || file.startsWith('dist/') },
  { name: 'local environment file', test: (file) => file.endsWith('/.env') || file === '.env' },
  { name: 'local agent state', test: (file) => file.endsWith('/.dockpilot-agent.json') || file === '.dockpilot-agent.json' },
  { name: 'runtime log file', test: (file) => /\.(log|pid)$/i.test(file) },
  { name: 'runtime database file', test: (file) => /\.(db|sqlite)(-(shm|wal))?$/i.test(file) || /\.(db|sqlite)-(shm|wal)$/i.test(file) },
  { name: 'local Compose backup', test: (file) => /^deploy\/.*\.bak-/.test(file) },
  { name: 'local Compose override', test: (file) => file === 'deploy/docker-compose.override.yml' || file === 'deploy/docker-compose.local.yml' }
]

const docPlanPatterns = [
  { name: 'plan heading', regex: /^# .*计划\s*$/im },
  { name: 'implementation prompt', regex: /PLEASE IMPLEMENT THIS PLAN/i },
  { name: 'plan summary heading', regex: /^## Summary\s*$/im },
  { name: 'plan key changes heading', regex: /^## Key Changes\s*$/im },
  { name: 'plan test heading', regex: /^## Test Plan\s*$/im },
  { name: 'plan assumptions heading', regex: /^## Assumptions\s*$/im },
  { name: 'creator trace', regex: /(ChatGPT|Codex|prompt|提示词|创作过程|设计方式)/i }
]

const publicDocs = new Set(['README.md'])
for (const file of files) {
  if (file.startsWith('docs/') && file.endsWith('.md')) {
    publicDocs.add(file)
  }
}

const failures = []
for (const file of files) {
  for (const check of blockedRuntimeFiles) {
    if (check.test(file)) {
      failures.push(`${file}: tracked ${check.name}`)
    }
  }

  const content = fs.readFileSync(file, 'utf8')
  for (const check of secretChecks) {
    if (check.test(content)) {
      failures.push(`${file}: contains ${check.name}`)
    }
  }
  if (publicDocs.has(file)) {
    for (const pattern of docPlanPatterns) {
      if (pattern.regex.test(content)) {
        failures.push(`${file}: contains ${pattern.name}`)
      }
    }
  }
}

if (failures.length > 0) {
  console.error('Public content check failed:')
  for (const failure of failures) {
    console.error(`- ${failure}`)
  }
  process.exit(1)
}

console.log('Public content check passed.')

function containsBlockedCredential(content) {
  for (const token of content.match(/\S{8,}/g) || []) {
    for (const candidate of token.split(/[=,:]/)) {
      const normalized = candidate.replace(/^[`'"]+|[`'",;]+$/g, '')
      if (blockedCredentialHashes.has(sha256(normalized))) {
        return true
      }
    }
  }
  return false
}

function sha256(value) {
  return crypto.createHash('sha256').update(value).digest('hex')
}

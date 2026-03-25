import { spawnSync } from 'child_process'

export default async function globalTeardown() {
  spawnSync('docker', ['rm', '-f', 'aerodocs-e2e'], { stdio: 'pipe' })
}

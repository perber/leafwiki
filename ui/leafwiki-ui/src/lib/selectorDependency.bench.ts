import { test } from 'vitest'
import { createRequire } from 'node:module'
const parser = createRequire(import.meta.url)(
  'postcss-selector-parser',
) as () => { processSync: (selector: string) => string }
for (const count of [1000, 5000, 10000]) {
  const selector = '.a'.repeat(count)
  test(`flat CSS selector: ${count} classes`, async ({ bench }) => {
    const result = await bench('parse', () => {
      parser().processSync(selector)
    }).run({ time: 1000, iterations: 10, warmupIterations: 1, warmupTime: 100 })
    console.log(
      JSON.stringify({
        classes: count,
        meanMs: result.latency.mean,
        rme: result.latency.rme,
      }),
    )
  }, 30000)
}

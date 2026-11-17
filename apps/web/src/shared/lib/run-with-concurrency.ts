/**
 * Runs async work in FIFO order with a fixed in-flight cap while preserving
 * input order in the settled results.
 */
export async function runWithConcurrency<T>(
  items: readonly T[],
  limit: number,
  worker: (item: T, index: number) => Promise<void>,
): Promise<PromiseSettledResult<void>[]> {
  const cap = Math.max(1, Math.floor(limit) || 1)
  const results: PromiseSettledResult<void>[] = new Array(items.length)
  let nextIndex = 0

  async function pump(): Promise<void> {
    while (true) {
      const index = nextIndex
      nextIndex += 1
      if (index >= items.length) return

      try {
        await worker(items[index], index)
        results[index] = { status: 'fulfilled', value: undefined }
      } catch (reason) {
        results[index] = { status: 'rejected', reason }
      }
    }
  }

  const workers = Array.from(
    { length: Math.min(cap, items.length) },
    () => pump(),
  )
  await Promise.all(workers)
  return results
}

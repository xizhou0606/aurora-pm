// 有界并发的 worker 池(原 generator.ts 内实现,逐字迁移):
// 成功释放的槽位可立即被补充 worker 复用,失败交给共享失败闩锁处理。
export async function mapWithConcurrency<T, R>(
  values: T[],
  concurrency: number,
  mapper: (value: T, index: number) => Promise<R>,
  onFirstError: () => void,
  startAdditionalWorkerWhen?: Promise<unknown>,
): Promise<R[]> {
  const results = new Array<R>(values.length);
  let nextIndex = 0;
  let failed = false;
  let firstError: unknown;
  const worker = async () => {
    while (!failed) {
      const index = nextIndex;
      nextIndex += 1;
      if (index >= values.length) return;
      try {
        results[index] = await mapper(values[index], index);
      } catch (error) {
        if (!failed) {
          failed = true;
          firstError = error;
          onFirstError();
        }
        return;
      }
    }
  };
  const workers = Array.from(
    { length: Math.min(concurrency, values.length) },
    worker,
  );
  if (startAdditionalWorkerWhen && workers.length < values.length) {
    // The released slot is usable only after its owner succeeds. A rejection
    // leaves failure selection and cancellation to the shared failure latch.
    workers.push(startAdditionalWorkerWhen.then(worker, () => undefined));
  }
  await Promise.all(workers);
  if (failed) throw firstError;
  return results;
}

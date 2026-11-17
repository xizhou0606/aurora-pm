import { describe, expect, it } from 'bun:test';
import { mapWithConcurrency } from './runtime/generate-concurrency.js';
import { shouldGenerateQATestCaseGlobalPlan } from './pipeline/generate-plan.js';
import { testCaseBatchConcurrency, TEST_CASE_BATCH_CONCURRENCY } from './runtime/generate-runtime.js';
import { partitionQATestCaseInput } from '../index.js';

// bun:test does not ship vi.waitFor; poll the same assertion to a deadline.
async function waitFor(assertion: () => void, timeoutMs = 1_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  let lastError: unknown;
  while (Date.now() < deadline) {
    try {
      assertion();
      return;
    } catch (error) {
      lastError = error;
      await new Promise((resolve) => setTimeout(resolve, 5));
    }
  }
  throw lastError;
}

describe('test-case generation', () => {
  it('skips Global Plan when one semantic requirement group spans multiple physical batches', () => {
    const input = {
      subject: { title: '单需求复杂测试' },
      test_case_context: {
        coverageRequired: true,
        requirements: [{ id: 141, title: '批量修改时间和负责人' }],
        tasks: Array.from({ length: 15 }, (_, index) => ({
          id: index + 1,
          requirement_id: 141,
          title: `任务 ${index + 1}`,
        })),
      },
    };
    const batches = partitionQATestCaseInput(input);

    expect(batches).toHaveLength(2);
    expect(shouldGenerateQATestCaseGlobalPlan(input, batches.length)).toBe(false);
    expect(testCaseBatchConcurrency(false)).toBe(TEST_CASE_BATCH_CONCURRENCY);
  });

  it('keeps single requirement document mode on one Draft call without a Global Plan', () => {
    const input = {
      subject: { title: '单需求文档测试' },
      sources: [{ content: '用户提交表单后应展示明确的成功结果。' }],
      test_case_context: {
        mode: 'single_requirement_document',
        coverageRequired: true,
        requirements: [{ id: 141, title: '单需求文档测试' }],
        tasks: [],
      },
    };
    const batches = partitionQATestCaseInput(input);

    expect(batches).toHaveLength(1);
    expect(batches[0].input).toBe(input);
    expect(shouldGenerateQATestCaseGlobalPlan(input, batches.length)).toBe(false);
  });

  it('retains Global Plan when multiple semantic requirement groups span batches', () => {
    const input = {
      subject: { title: '跨需求复杂测试' },
      test_case_context: {
        coverageRequired: true,
        requirements: Array.from({ length: 6 }, (_, index) => ({
          id: index + 1,
          title: `需求 ${index + 1}`,
        })),
        tasks: Array.from({ length: 6 }, (_, index) => ({
          id: index + 101,
          requirement_id: index + 1,
          title: `任务 ${index + 1}`,
        })),
      },
    };
    const batches = partitionQATestCaseInput(input);

    expect(batches).toHaveLength(2);
    expect(shouldGenerateQATestCaseGlobalPlan(input, batches.length)).toBe(true);
    expect(testCaseBatchConcurrency(true)).toBe(TEST_CASE_BATCH_CONCURRENCY - 1);
  });

  it('does not treat orphan task groups as formal cross-requirement groups', () => {
    const input = {
      subject: { title: '任务集合测试' },
      test_case_context: {
        coverageRequired: true,
        requirements: [],
        tasks: Array.from({ length: 15 }, (_, index) => ({
          id: index + 1,
          requirement_id: index + 1000,
          title: `未关联任务 ${index + 1}`,
        })),
      },
    };
    const batches = partitionQATestCaseInput(input);

    expect(batches).toHaveLength(2);
    expect(shouldGenerateQATestCaseGlobalPlan(input, batches.length)).toBe(false);
  });

  it('still rejects duplicate canonical requirement IDs before selecting a Global Plan', () => {
    const input = {
      test_case_context: {
        requirements: [{ id: 1, title: '需求一' }, { id: '1', title: '重复需求' }],
        tasks: Array.from({ length: 9 }, (_, index) => ({ title: `任务 ${index + 1}` })),
      },
    };

    expect(() => shouldGenerateQATestCaseGlobalPlan(input, 2))
      .toThrow('qa test case context has duplicate canonical requirement IDs');
  });

  it('releases the successful Global slot to a fourth Draft worker without exceeding total concurrency four', async () => {
    let releaseGlobal!: () => void;
    const globalSlot = new Promise<void>((resolve) => { releaseGlobal = resolve; });
    const completeDraft: Array<((value: string) => void) | undefined> = [];
    const started: number[] = [];
    let globalActive = 1;
    let activeDrafts = 0;
    let peakTotalConcurrency = 0;
    const recordPeak = () => {
      peakTotalConcurrency = Math.max(peakTotalConcurrency, globalActive + activeDrafts);
    };
    recordPeak();

    const scheduled = mapWithConcurrency(
      [0, 1, 2, 3],
      testCaseBatchConcurrency(true),
      async (_value, index) => {
        started.push(index);
        activeDrafts += 1;
        recordPeak();
        const result = await new Promise<string>((resolve) => { completeDraft[index] = resolve; });
        activeDrafts -= 1;
        return result;
      },
      () => undefined,
      globalSlot,
    );

    expect(started).toEqual([0, 1, 2]);
    expect(peakTotalConcurrency).toBe(4);

    globalActive = 0;
    releaseGlobal();
    await waitFor(() => { expect(started).toEqual([0, 1, 2, 3]); });
    expect(activeDrafts).toBe(4);
    expect(peakTotalConcurrency).toBe(4);

    completeDraft[0]?.('batch-0');
    completeDraft[3]?.('batch-3');
    completeDraft[2]?.('batch-2');
    completeDraft[1]?.('batch-1');

    await expect(scheduled).resolves.toEqual(['batch-0', 'batch-1', 'batch-2', 'batch-3']);
    expect(peakTotalConcurrency).toBe(4);
  });
});

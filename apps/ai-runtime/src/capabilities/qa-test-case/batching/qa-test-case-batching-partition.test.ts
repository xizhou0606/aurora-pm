import { describe, expect, it } from 'bun:test';
import {
  partitionQATestCaseInput,
  QA_TEST_CASE_BATCH_MAX_BYTES,
  QA_TEST_CASE_BATCH_MAX_ITEMS,
  QA_TEST_CASE_MAX_BATCHES,
  QA_TEST_CASE_MAX_ITEMS,
} from './qa-test-case-batching-partition.js';
import { complexInput } from './qa-test-case-batching-fixtures.js';

describe('partitionQATestCaseInput', () => {
  it('keeps the fixed 120-item capacity in twelve compact-IR batches', () => {
    expect(QA_TEST_CASE_BATCH_MAX_ITEMS).toBe(10);
    expect(QA_TEST_CASE_MAX_BATCHES).toBe(12);
    expect(QA_TEST_CASE_MAX_ITEMS).toBe(120);
  });

  it('keeps the single-batch path unchanged', () => {
    const input = complexInput(2);
    const batches = partitionQATestCaseInput(input);

    expect(batches).toHaveLength(1);
    expect(batches[0].input).toBe(input);
    expect(batches[0].globalIndices).toEqual({ requirements: [0, 1], tasks: [0, 1], sources: [0, 1, 2] });
  });

  it('rejects task context in single requirement document mode', () => {
    const input = complexInput(1) as Record<string, unknown>;
    const context = input.test_case_context as Record<string, unknown>;
    context.mode = 'single_requirement_document';

    expect(() => partitionQATestCaseInput(input))
      .toThrow('single requirement document mode does not accept task context');
  });

  it('keeps related requirements and tasks in bounded deterministic batches', () => {
    const batches = partitionQATestCaseInput(complexInput(25));

    expect(batches).toHaveLength(5);
    expect(batches.length).toBeLessThanOrEqual(QA_TEST_CASE_MAX_BATCHES);
    for (const batch of batches) {
      const context = batch.input.test_case_context as {
        requirements: unknown[];
        tasks: unknown[];
        counts: { requirements: number; tasks: number };
      };
      expect(context.requirements.length + context.tasks.length).toBeLessThanOrEqual(QA_TEST_CASE_BATCH_MAX_ITEMS);
      expect(context.counts).toEqual({
        requirements: context.requirements.length,
        tasks: context.tasks.length,
      });
    }
    expect(batches[0].globalIndices).toEqual({
      requirements: Array.from({ length: 5 }, (_, index) => index),
      tasks: Array.from({ length: 5 }, (_, index) => index),
      sources: [...Array.from({ length: 5 }, (_, index) => index), 25],
    });
    expect(batches[1].globalIndices.sources).toEqual([
      ...Array.from({ length: 5 }, (_, index) => index + 5), 25,
    ]);

    const maximum = partitionQATestCaseInput(complexInput(QA_TEST_CASE_MAX_ITEMS / 2));
    expect(maximum).toHaveLength(QA_TEST_CASE_MAX_BATCHES);
    expect(maximum.every((batch) => {
      const context = batch.input.test_case_context as { requirements: unknown[]; tasks: unknown[] };
      return context.requirements.length + context.tasks.length === QA_TEST_CASE_BATCH_MAX_ITEMS;
    })).toBe(true);
  });

  it('keeps related pairs together before balancing the minimum batch count', () => {
    const batches = partitionQATestCaseInput(complexInput(13));
    const sizes = batches.map((batch) => {
      const context = batch.input.test_case_context as {
        requirements: unknown[];
        tasks: unknown[];
      };
      return context.requirements.length + context.tasks.length;
    });

    expect(batches).toHaveLength(3);
    expect(sizes).toEqual([10, 8, 8]);
    expect(batches.flatMap((batch) => batch.globalIndices.requirements))
      .toEqual(Array.from({ length: 13 }, (_, index) => index));
    expect(batches.flatMap((batch) => batch.globalIndices.tasks))
      .toEqual(Array.from({ length: 13 }, (_, index) => index));
    for (let index = 0; index < 13; index += 1) {
      expect(batches.findIndex((batch) => batch.globalIndices.requirements.includes(index)))
        .toBe(batches.findIndex((batch) => batch.globalIndices.tasks.includes(index)));
    }
  });

  it('uses the compiler ledger to match numeric requirement IDs to string task links', () => {
    const input = complexInput(13);
    const context = input.test_case_context;
    (context.tasks as Array<{ requirement_id: string | number }>)[3].requirement_id = String(
      context.requirements[3].id,
    );

    const batches = partitionQATestCaseInput(input);

    expect(batches.findIndex((batch) => batch.globalIndices.requirements.includes(3)))
      .toBe(batches.findIndex((batch) => batch.globalIndices.tasks.includes(3)));
  });

  it('rejects duplicate canonical requirement IDs before partitioning can hide ambiguity', () => {
    const input = {
      sources: [
        { url: 'https://example.feishu.cn/docx/duplicate-a', content: '重复需求 A 正文' },
        { url: 'https://example.feishu.cn/docx/duplicate-b', content: '重复需求 B 正文' },
      ],
      test_case_context: {
        requirements: Array.from({ length: 8 }, (_, index) => ({
          id: index < 2 ? (index === 0 ? 1 : '1') : index + 1,
          title: `需求 ${index + 1}`,
          ...(index < 2 && {
            product_doc_url: `https://example.feishu.cn/docx/duplicate-${index === 0 ? 'a' : 'b'}`,
          }),
        })),
        tasks: [{ id: 101, requirement_id: 1, title: '归属不明的任务' }],
      },
    };

    expect(() => partitionQATestCaseInput(input))
      .toThrow('qa test case context has duplicate canonical requirement IDs');
  });

  it('keeps adjacent tasks with the same orphan requirement ID together when balancing', () => {
    const input = {
      test_case_context: {
        requirements: Array.from({ length: 4 }, (_, index) => ({
          id: index + 1,
          title: `需求 ${index + 1}`,
        })),
        tasks: [
          { id: 101, requirement_id: 99, title: '孤儿任务一' },
          { id: 102, requirement_id: '99', title: '孤儿任务二' },
          { id: 103, requirement_id: 100, title: '孤儿任务三' },
          { id: 104, requirement_id: 101, title: '孤儿任务四' },
          { id: 105, requirement_id: 102, title: '孤儿任务五' },
        ],
      },
    };

    const batches = partitionQATestCaseInput(input);

    expect(batches).toHaveLength(1);
    expect(batches.findIndex((batch) => batch.globalIndices.tasks.includes(0)))
      .toBe(batches.findIndex((batch) => batch.globalIndices.tasks.includes(1)));
    expect(batches.flatMap((batch) => batch.globalIndices.tasks)).toEqual([0, 1, 2, 3, 4]);
  });

  it('keeps a requirement source in task-only continuation batches', () => {
    const input = {
      sources: [
        { url: 'https://example.feishu.cn/docx/requirement-1', content: '需求正文' },
        { url: 'https://example.feishu.cn/docx/global', content: '全局规则' },
      ],
      test_case_context: {
        requirements: [{
          id: 1,
          title: '需求 1',
          product_doc_url: 'https://example.feishu.cn/docx/requirement-1#section',
        }],
        tasks: Array.from({ length: 30 }, (_, index) => ({
          id: index + 1, requirement_id: 1, title: `任务 ${index + 1}`,
        })),
      },
    };

    const batches = partitionQATestCaseInput(input);

    expect(batches).toHaveLength(4);
    expect(batches.map((batch) => (
      batch.globalIndices.requirements.length + batch.globalIndices.tasks.length
    ))).toEqual([8, 8, 8, 7]);
    expect(batches[1].globalIndices.requirements).toEqual([]);
    expect(batches[1].globalIndices.sources).toEqual([0, 1]);
    expect((batches[1].input.sources as unknown[])).toHaveLength(2);
    expect(batches.every((batch) => (
      batch.globalIndices.sources.join(',') === '0,1'
    ))).toBe(true);
  });

  it('partitions a source bundle above 2 MiB before model generation', () => {
    const input = complexInput(3);
    input.sources = input.sources.map((source, index) => ({
      ...source,
      content: index < 3 ? String(index).repeat(750 << 10) : source.content,
    }));

    expect(Buffer.byteLength(JSON.stringify(input), 'utf8')).toBeGreaterThan(2 << 20);
    const batches = partitionQATestCaseInput(input);

    expect(batches).toHaveLength(3);
    expect(batches.map((batch) => (
      batch.globalIndices.requirements.length + batch.globalIndices.tasks.length
    ))).toEqual([2, 2, 2]);
    expect(batches.every((batch) => (
      Buffer.byteLength(JSON.stringify(batch.input), 'utf8') <= QA_TEST_CASE_BATCH_MAX_BYTES
    ))).toBe(true);
    expect(batches.map((batch) => batch.globalIndices.sources)).toEqual([
      [0, 3], [1, 3], [2, 3],
    ]);
  });

  it('rejects total item and indivisible byte overflows before generation', () => {
    expect(() => partitionQATestCaseInput({
      test_case_context: {
        requirements: Array.from({ length: QA_TEST_CASE_MAX_ITEMS + 1 }, (_, id) => ({ id })),
        tasks: [],
      },
    })).toThrow(`maximum supported is ${QA_TEST_CASE_MAX_ITEMS}`);

    expect(() => partitionQATestCaseInput({
      test_case_context: {
        requirements: [{ id: 1, description: 'x'.repeat(QA_TEST_CASE_BATCH_MAX_BYTES) }],
        tasks: [],
      },
    })).toThrow(/exceeds the .*byte batch limit/);
  });
});

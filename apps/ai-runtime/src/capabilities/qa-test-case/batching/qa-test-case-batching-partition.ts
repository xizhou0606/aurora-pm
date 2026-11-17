import { qaTestCaseRequirementGroupLedger } from '../draft/qa-test-case-draft-context.js';
import {
  batchInput,
  byteLength,
  record,
  relatedContextItems,
  type ContextItem,
  type QATestCaseBatch,
} from './qa-test-case-batching-input.js';

export const QA_TEST_CASE_BATCH_MAX_ITEMS = 10;
export const QA_TEST_CASE_BATCH_MAX_BYTES = 1024 * 1024;
export const QA_TEST_CASE_MAX_BATCHES = 12;
export const QA_TEST_CASE_MAX_ITEMS = QA_TEST_CASE_BATCH_MAX_ITEMS * QA_TEST_CASE_MAX_BATCHES;

type BatchCandidate = Pick<QATestCaseBatch, 'input' | 'globalIndices'> & {
  end: number;
};

function feasibleBatchCandidates(
  input: Record<string, unknown>,
  context: Record<string, unknown>,
  requirements: unknown[],
  tasks: unknown[],
  items: ContextItem[],
): BatchCandidate[][] {
  return items.map((_item, start) => {
    const candidates: BatchCandidate[] = [];
    const maximumEnd = Math.min(items.length, start + QA_TEST_CASE_BATCH_MAX_ITEMS);
    for (let end = start + 1; end <= maximumEnd; end += 1) {
      const built = batchInput(input, context, requirements, tasks, items.slice(start, end));
      if (byteLength(built.input) <= QA_TEST_CASE_BATCH_MAX_BYTES) {
        candidates.push({ end, ...built });
      }
    }
    return candidates;
  });
}

function minimumBatchCount(candidates: BatchCandidate[][], itemCount: number): number[] {
  const minimum = Array<number>(itemCount + 1).fill(Number.POSITIVE_INFINITY);
  minimum[itemCount] = 0;
  for (let start = itemCount - 1; start >= 0; start -= 1) {
    for (const candidate of candidates[start]) {
      minimum[start] = Math.min(minimum[start], 1 + minimum[candidate.end]);
    }
  }
  return minimum;
}

function balancedBatchCandidates(
  candidates: BatchCandidate[][],
  items: ContextItem[],
  itemCount: number,
  batchCount: number,
): BatchCandidate[] {
  type BalancedResult = {
    groupSplits: number;
    varianceCost: number;
    batches: BatchCandidate[];
  };
  const memo = new Map<string, BalancedResult | null>();
  const solve = (
    start: number,
    batchIndex: number,
  ): BalancedResult | null => {
    if (batchIndex === batchCount) {
      return start === itemCount
        ? { groupSplits: 0, varianceCost: 0, batches: [] }
        : null;
    }
    const key = `${start}:${batchIndex}`;
    if (memo.has(key)) return memo.get(key)!;
    const remainingBatches = batchCount - batchIndex - 1;
    let best: BalancedResult | null = null;
    for (const candidate of candidates[start] ?? []) {
      const remainingItems = itemCount - candidate.end;
      if (remainingItems < remainingBatches
        || remainingItems > remainingBatches * QA_TEST_CASE_BATCH_MAX_ITEMS) continue;
      const suffix = solve(candidate.end, batchIndex + 1);
      if (!suffix) continue;
      const size = candidate.end - start;
      const splitsGroup = candidate.end < itemCount
        && items[candidate.end - 1].group === items[candidate.end].group;
      const groupSplits = Number(splitsGroup) + suffix.groupSplits;
      // Preserve related groups first. Within that optimum, fixed total/count
      // means minimizing squared sizes minimizes variance.
      const varianceCost = size ** 2 + suffix.varianceCost;
      if (!best || groupSplits < best.groupSplits
        || (groupSplits === best.groupSplits && varianceCost < best.varianceCost)
        || (groupSplits === best.groupSplits && varianceCost === best.varianceCost
          && size > best.batches[0].end - start)) {
        best = { groupSplits, varianceCost, batches: [candidate, ...suffix.batches] };
      }
    }
    memo.set(key, best);
    return best;
  };
  return solve(0, 0)?.batches ?? [];
}

/**
 * Split the formal requirement/task collections and retain only sources that
 * belong to the selected requirements, plus sources that cannot be associated
 * safely. Every local array index is recorded for deterministic evidence repair.
 */
export function partitionQATestCaseInput(input: Record<string, unknown>): QATestCaseBatch[] {
  const context = record(input.test_case_context);
  const requirements = Array.isArray(context.requirements) ? context.requirements : [];
  const tasks = Array.isArray(context.tasks) ? context.tasks : [];
  const totalItems = requirements.length + tasks.length;
  const sources = Array.isArray(input.sources) ? input.sources : [];
  const identityIndices = {
    requirements: [...requirements.keys()],
    tasks: [...tasks.keys()],
    sources: [...sources.keys()],
  };

  if (context.mode === 'single_requirement_document') {
    if (requirements.length !== 1) {
      throw new Error('single requirement document mode requires exactly one requirement');
    }
    if (tasks.length !== 0) {
      throw new Error('single requirement document mode does not accept task context');
    }
    if (byteLength(input) > QA_TEST_CASE_BATCH_MAX_BYTES) {
      throw new Error(`single requirement document exceeds the ${QA_TEST_CASE_BATCH_MAX_BYTES}-byte generation limit`);
    }
    return [{ index: 0, input, globalIndices: identityIndices }];
  }

  if (totalItems === 0) return [{ index: 0, input, globalIndices: identityIndices }];
  if (totalItems > QA_TEST_CASE_MAX_ITEMS) {
    throw new Error(`test case context has ${totalItems} items; maximum supported is ${QA_TEST_CASE_MAX_ITEMS}`);
  }
  const requirementGroupLedger = qaTestCaseRequirementGroupLedger(input);
  if (totalItems <= QA_TEST_CASE_BATCH_MAX_ITEMS && byteLength(input) <= QA_TEST_CASE_BATCH_MAX_BYTES) {
    return [{ index: 0, input, globalIndices: identityIndices }];
  }

  const commonInput = batchInput(input, context, requirements, tasks, []);
  if (byteLength(commonInput.input) > QA_TEST_CASE_BATCH_MAX_BYTES) {
    throw new Error(`test case common context exceeds the ${QA_TEST_CASE_BATCH_MAX_BYTES}-byte batch limit`);
  }

  const items = relatedContextItems(requirements, tasks, requirementGroupLedger);
  const candidates = feasibleBatchCandidates(input, context, requirements, tasks, items);
  for (const [index, item] of items.entries()) {
    if (!candidates[index].some((candidate) => candidate.end === index + 1)) {
      throw new Error(`test case context item ${item.collection}[${item.globalIndex}] exceeds the ${QA_TEST_CASE_BATCH_MAX_BYTES}-byte batch limit`);
    }
  }
  const minimum = minimumBatchCount(candidates, items.length)[0];
  if (!Number.isFinite(minimum) || minimum > QA_TEST_CASE_MAX_BATCHES) {
    const required = Number.isFinite(minimum) ? minimum : items.length;
    throw new Error(`test case context requires ${required} batches; maximum supported is ${QA_TEST_CASE_MAX_BATCHES}`);
  }
  const balanced = balancedBatchCandidates(candidates, items, items.length, minimum);
  if (balanced.length !== minimum) {
    throw new Error('test case context cannot be partitioned within the supported batch limits');
  }
  return balanced.map(({ input: batch, globalIndices }, index) => ({
    index,
    input: batch,
    globalIndices,
  }));
}

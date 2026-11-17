import { describe, expect, it } from 'bun:test';
import { batchInput } from './qa-test-case-batching-input.js';

describe('document source ownership', () => {
  it('keeps only sources belonging to selected requirements and task parents', () => {
    const requirements = [{ id: 10 }, { id: 20 }];
    const tasks = [{ id: 1, requirement_id: 20 }];
    const input = { sources: [
      { type: 'internal_document', requirement_ids: [10], content: 'A' },
      { type: 'internal_document', requirement_ids: [20], content: 'B' },
      { type: 'feishu_document', requirement_ids: [10, 20], content: 'shared' },
      { type: 'internal_document', content: 'unowned' },
    ] };
    const first = batchInput(input, {}, requirements, tasks, [{ collection: 'requirements', globalIndex: 0, group: 'requirement:0' }]);
    expect(first.globalIndices.sources).toEqual([0, 2]);
    const second = batchInput(input, {}, requirements, tasks, [{ collection: 'tasks', globalIndex: 0, group: 'requirement:1' }]);
    expect(second.globalIndices.sources).toEqual([1, 2]);
  });
});

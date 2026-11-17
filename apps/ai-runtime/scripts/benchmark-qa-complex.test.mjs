import { describe, expect, it } from 'vitest';
import { benchmarkInput, evaluateBenchmarkResponse } from './benchmark-qa-complex.mjs';

const skillProvenance = {
  commit: 'd75606531f6ce414bbcd95fb203033ae86653852',
  packageSha256: '07e9002dfe456988164f237f4cc33acce1a203564d1c778e94b3fe2bbc43e935',
  mode: 'advanced',
};

function successfulDecoded() {
  return {
    data: {
      output: {
        testCases: Array.from({ length: 13 }, (_, index) => ({
          evidence: [
            `test_case_context.requirements.${index}.description`,
            `test_case_context.tasks.${index}.description`,
          ],
        })),
        testCaseDesign: { objective: 'benchmark' },
        skillProvenance,
      },
      usage: { inputTokens: 100, outputTokens: 50, totalTokens: 150 },
    },
  };
}

describe('complex QA benchmark gate', () => {
  it('constructs exactly 13 requirements and 13 linked tasks', () => {
    const input = benchmarkInput();

    expect(input.test_case_context.requirements).toHaveLength(13);
    expect(input.test_case_context.tasks).toHaveLength(13);
    expect(input.test_case_context.tasks.every((task, index) => (
      task.requirement_id === input.test_case_context.requirements[index].id
    ))).toBe(true);
  });

  it('passes only a complete response using the locked advanced skill', () => {
    const report = evaluateBenchmarkResponse({
      status: 200,
      decoded: successfulDecoded(),
      elapsedMs: 123,
      responseBytes: 456,
    });

    expect(report).toMatchObject({
      benchmarkPassed: true,
      failureCodes: [],
      caseCount: 13,
      requirementCoverage: 13,
      taskCoverage: 13,
    });
  });

  it('fails closed with finite codes and never returns response content', () => {
    const decoded = successfulDecoded();
    decoded.message = '机密服务端错误';
    decoded.data.usage.prompt = '机密 prompt';
    decoded.data.output.testCases = [];
    decoded.data.output.testCaseDesign = null;
    decoded.data.output.skillProvenance = {
      commit: 'wrong',
      packageSha256: 'wrong',
      mode: 'standard',
    };

    const report = evaluateBenchmarkResponse({
      status: 502,
      decoded,
      elapsedMs: 123,
      responseBytes: 456,
    });

    expect(report.benchmarkPassed).toBe(false);
    expect(report.failureCodes).toEqual([
      'http_status',
      'empty_cases',
      'missing_test_case_design',
      'requirement_coverage',
      'task_coverage',
      'skill_commit',
      'skill_package_sha256',
      'skill_mode',
    ]);
    expect(JSON.stringify(report)).not.toContain('机密服务端错误');
    expect(JSON.stringify(report)).not.toContain('机密 prompt');
    expect(JSON.stringify(report)).not.toContain('wrong');
  });

  it('fails when either context collection is only partially covered', () => {
    const decoded = successfulDecoded();
    decoded.data.output.testCases.at(-1).evidence = [];

    const report = evaluateBenchmarkResponse({
      status: 200,
      decoded,
      elapsedMs: 123,
      responseBytes: 456,
    });

    expect(report.benchmarkPassed).toBe(false);
    expect(report.failureCodes).toEqual(['requirement_coverage', 'task_coverage']);
    expect(report.missingRequirementIndexes).toEqual([12]);
    expect(report.missingTaskIndexes).toEqual([12]);
  });
});

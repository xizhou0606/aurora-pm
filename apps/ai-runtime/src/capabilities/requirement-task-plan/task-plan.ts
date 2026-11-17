import { aiOutputSchema, type AIOutput } from '../../shared/ai/schema.js';
import { taskPlanGenerationSchema } from './schema.js';

// requirement.task_plan 输出归一(原 generator.ts 内实现,逐字迁移):把两条
// 研发链路展开成通用 items 展示,并保留完整 taskPlan 结构。
export function normalizeTaskPlanGenerationOutput(value: unknown): AIOutput {
  const parsed = taskPlanGenerationSchema.parse(value);
  return aiOutputSchema.parse({
    summary: parsed.summary,
    changes: [],
    items: parsed.taskPlan.chains.flatMap((chain) => chain.tasks.map((task) => ({
      title: `${chain.lane === 'frontend' ? '前端' : '后端'} · ${task.title}`,
      description: task.description,
      evidence: task.acceptanceCriteria,
    }))),
    risks: [],
    testCases: [],
    humanReviewRequired: true,
    completenessReport: null,
    workflowProposal: null,
    taskPlan: parsed.taskPlan,
  });
}

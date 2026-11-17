import { z } from 'zod';
import { taskPlanSchema } from '../../shared/ai/schema.js';

// requirement.task_plan 的模型输出契约(原 src/schema.ts 内定义,随能力迁入
// 本目录;taskPlan 本体仍是共享输出信封 shared/ai/schema.ts 的一部分)。
export const taskPlanGenerationSchema = z.object({
  summary: z.string().min(1),
  taskPlan: taskPlanSchema,
});

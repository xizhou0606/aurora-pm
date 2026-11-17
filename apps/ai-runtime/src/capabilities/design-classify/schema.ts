import { z } from 'zod';

// requirement.design_classify 的模型输出契约(原 src/schema.ts 内定义,随能力
// 迁入本目录)。
export const designClassificationSchema = z.object({
  summary: z.string().min(1),
  requiresDesign: z.boolean(),
  evidence: z.array(z.string()),
  missingMaterials: z.array(z.string()),
  risks: z.array(z.string()),
});

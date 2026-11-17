import { z } from 'zod';

// code.commit_quality_review 的模型输出契约(原 src/schema.ts 内定义,随能力
// 迁入本目录;逐提交结论会经 normalizeCommitQualityReviewOutput 归一后写进
// AIOutput 的 commitReviews/qualityAssessment 扩展字段)。
export const commitQualityReviewOutputSchema = z.object({
  summary: z.string().min(1),
  changes: z.array(z.object({ field: z.string() })).default([]),
  commitReviews: z.array(
    z.object({
      sha: z.string(),
      verdict: z.enum(['优', '良', '中', '差']),
      summary: z.string().default(''),
      issues: z.array(z.string()).default([]),
    }),
  ),
  qualityAssessment: z.object({
    score: z.number().min(0).max(100),
    conclusion: z.string().default(''),
    commonIssues: z.array(z.string()).default([]),
    risks: z.array(z.string()).default([]),
    suggestions: z.array(z.string()).default([]),
  }),
});

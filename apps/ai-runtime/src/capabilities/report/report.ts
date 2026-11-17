// 报告产出解析(report.*):模型偶尔会无视"只输出 JSON"的指令,能兜出
// summary 就不浪费这次生成。产出 = summary 正文 + overall_comment(针对工作
// 情况、代码产出与代码质量的综合评语)+ overall_score(0-100 综合评分)+
// dimension_comments(任务/质量/代码贡献三个排行的维度短评:团队汇报各段
// 非空,个人汇报为空字符串)。旧格式只有 summary 时其余字段为空/缺省。

export type ReportDimensionComments = { tasks: string; quality: string; code: string };

const EMPTY_DIMENSION_COMMENTS: ReportDimensionComments = { tasks: '', quality: '', code: '' };

function dimensionComments(value: unknown): ReportDimensionComments {
  if (value === null || typeof value !== 'object') return { ...EMPTY_DIMENSION_COMMENTS };
  const entry = value as Record<string, unknown>;
  const text = (key: string): string => (typeof entry[key] === 'string' ? (entry[key] as string).trim() : '');
  return { tasks: text('tasks'), quality: text('quality'), code: text('code') };
}

// 模型可能输出小数或越界分数：收敛为 0-100 的整数，非法值视为缺失。
function overallScore(value: unknown): number | undefined {
  if (typeof value !== 'number' || !Number.isFinite(value)) return undefined;
  return Math.min(100, Math.max(0, Math.round(value)));
}

function safeJsonParse(text: string): Record<string, unknown> | null {
  try {
    const parsed = JSON.parse(text) as unknown;
    return parsed !== null && typeof parsed === 'object' ? (parsed as Record<string, unknown>) : null;
  } catch {
    return null;
  }
}

// 修复字符串值内未转义的双引号（模型偶发）：按结构位置判定——引号的前一个
// 非空白字符属于 { , : [ 或后一个非空白字符属于 : , } ] 时视为结构引号，
// 其余一律转义。仅在严格解析失败后作为兜底，尽力而为。
function escapeInnerQuotes(raw: string): string {
  let out = '';
  for (let index = 0; index < raw.length; index += 1) {
    const char = raw[index];
    if (char !== '"') {
      out += char;
      continue;
    }
    let prevIndex = out.length - 1;
    while (prevIndex >= 0 && /\s/.test(out[prevIndex])) prevIndex -= 1;
    const prev = prevIndex >= 0 ? out[prevIndex] : '';
    let nextIndex = index + 1;
    while (nextIndex < raw.length && /\s/.test(raw[nextIndex])) nextIndex += 1;
    const next = nextIndex < raw.length ? raw[nextIndex] : '';
    const structural = prev === '' || prev === '\\' || '{[,:'.includes(prev) || ':,}]'.includes(next);
    out += structural ? '"' : '\\"';
  }
  return out;
}

function parseLooseObject(trimmed: string): Record<string, unknown> | null {
  if (!trimmed.startsWith('{')) return null;
  return safeJsonParse(trimmed) ?? safeJsonParse(escapeInnerQuotes(trimmed));
}

export function parseReportOutput(rawText: string): {
  summary: string;
  overallComment: string;
  overallScore?: number;
  dimensionComments: ReportDimensionComments;
} {
  const trimmed = rawText.trim();
  if (!trimmed) return { summary: '', overallComment: '', dimensionComments: { ...EMPTY_DIMENSION_COMMENTS } };
  const parsed = parseLooseObject(trimmed);
  if (parsed && typeof parsed.summary === 'string' && parsed.summary.trim()) {
    return {
      summary: parsed.summary.trim(),
      overallComment: typeof parsed.overall_comment === 'string' ? parsed.overall_comment.trim() : '',
      overallScore: overallScore(parsed.overall_score),
      dimensionComments: dimensionComments(parsed.dimension_comments),
    };
  }
  const fenced = /```(?:markdown)?\s*([\s\S]*?)```/.exec(trimmed);
  return { summary: (fenced?.[1] ?? trimmed).trim(), overallComment: '', dimensionComments: { ...EMPTY_DIMENSION_COMMENTS } };
}

/** 模型偶尔会无视“只输出 JSON”的指令；能兜出 summary 就不浪费这次生成。 */
export function parseReportSummary(rawText: string): string {
  return parseReportOutput(rawText).summary;
}

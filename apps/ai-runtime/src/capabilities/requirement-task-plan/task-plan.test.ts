import { describe, expect, it } from 'bun:test';
import { normalizeTaskPlanGenerationOutput } from './task-plan.js';
import { validateCapabilityOutput } from '../executor.js';
import { buildTaskPlanPrompt } from './prompt.js';

describe('validateCapabilityOutput', () => {
	it('normalizes a strict frontend/backend task plan', () => {
		const output = normalizeTaskPlanGenerationOutput({
			summary: '已拆分前后端研发任务',
			taskPlan: { chains: [
				{ lane: 'frontend', summary: '前端交付', tasks: [{ title: '实现页面', description: '完成页面与接口接入', acceptanceCriteria: ['页面可操作'] }] },
				{ lane: 'backend', summary: '后端交付', tasks: [{ title: '实现接口', description: '完成接口与持久化', acceptanceCriteria: ['接口测试通过'] }] },
			] },
		});

		expect(output.taskPlan?.chains.map((chain) => chain.lane)).toEqual(['frontend', 'backend']);
		expect(output.items.map((item) => item.title)).toEqual(['前端 · 实现页面', '后端 · 实现接口']);
		expect(() => validateCapabilityOutput('requirement.task_plan', { subject: { id: 134 } }, output)).not.toThrow();
	});

	it('rejects a task plan without both delivery chains', () => {
		expect(() => normalizeTaskPlanGenerationOutput({
			summary: '只有前端',
			taskPlan: { chains: [
				{ lane: 'frontend', summary: '前端一', tasks: [{ title: '页面一', description: '实现页面一', acceptanceCriteria: ['完成'] }] },
				{ lane: 'frontend', summary: '前端二', tasks: [{ title: '页面二', description: '实现页面二', acceptanceCriteria: ['完成'] }] },
			] },
		})).toThrow('task plan must contain one frontend and one backend chain');
	});
});

describe('buildTaskPlanPrompt', () => {
  it('requires exactly one frontend and one backend task chain', () => {
    const prompt = buildTaskPlanPrompt({ subject: { id: 134, title: '结算优化' } });

    expect(prompt).toContain('lane="frontend"');
    expect(prompt).toContain('lane="backend"');
    expect(prompt).toContain('分别创建为父任务');
    expect(prompt).toContain('只生成待确认的任务方案');
    expect(prompt).toContain('人工确认采用后');
    expect(prompt).not.toContain('全部自动关联当前需求');
  });
});

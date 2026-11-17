package aiworker

import (
	"context"
	"errors"
	"fmt"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	"strings"
	"time"
)

func runWithSourceDeadline(ctx context.Context, timeout time.Duration, timeoutMessage string, operation func(context.Context) error) error {
	boundedCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err := operation(boundedCtx)
	if boundedCtx.Err() == context.DeadlineExceeded {
		return errors.New(timeoutMessage)
	}
	return err
}

func (w *Worker) verifyQATestCaseSourceFreshness(ctx context.Context, run *aigorm.AIRun) error {
	if run.Capability != "qa.test_cases" {
		return nil
	}

	var sources []aigorm.AIRunSource
	if err := w.db.WithContext(ctx).Where(
		"workspace_id = ? AND run_id = ? AND source_type = ?",
		run.WorkspaceID,
		run.ID,
		"feishu_link",
	).Order("id").Find(&sources).Error; err != nil {
		return fmt.Errorf("load Feishu source snapshots: %w", err)
	}
	if len(sources) == 0 {
		return nil
	}
	if !w.cfg.Feishu.Enabled || w.reader == nil {
		return fmt.Errorf("飞书需求来源无法复核，测试用例不会保存；请恢复飞书读取配置后重试")
	}

	for _, source := range sources {
		if strings.TrimSpace(source.Revision) == "" || strings.TrimSpace(source.ContentHash) == "" {
			return fmt.Errorf("飞书需求来源缺少生成前快照，测试用例不会保存；请重新生成")
		}
		document, err := w.reader.Read(ctx, source.SourceURL)
		if err != nil {
			return fmt.Errorf("生成后复核飞书需求来源失败，测试用例不会保存；请重试")
		}
		if strings.TrimSpace(document.Content) == "" {
			return fmt.Errorf("生成后复核的飞书需求正文为空，测试用例不会保存；请重试")
		}
		if document.ContentTruncated || document.BlockRefsTruncated {
			return fmt.Errorf("生成后复核的飞书需求正文或块引用超过读取上限，无法保证测试用例完整性；请拆分文档后重试")
		}
		if document.Revision != source.Revision || document.ContentHash != source.ContentHash {
			return fmt.Errorf("飞书需求文档在测试用例生成期间发生变化，测试用例不会保存；请重新生成")
		}
	}
	return nil
}

package feishu

import "testing"

func elementsOf(t *testing.T, content string) []mdElement {
	t.Helper()
	elements := parseMarkdownElements(content)
	if len(elements) == 0 {
		t.Fatalf("content must parse into elements:\n%s", content)
	}
	return elements
}

func TestParseMarkdownReportStructure(t *testing.T) {
	elements := elementsOf(t, "> 📌 对象：张三\n\n# 周报 · 张三 · 2026-09-06\n\n## 📊 数据指标\n\n| 指标 | 数量 |\n| --- | --- |\n| 完成任务 | 3 |\n\n- 完成 **登录改造**\n- 修复 `越权` 问题\n\n1. 第一步\n\n---\n\n普通段落\n")
	if _, ok := elements[0].(mdCallout); !ok {
		t.Fatalf("quote line must become a callout, got %T", elements[0])
	}
	heading, ok := elements[1].(mdHeading)
	if !ok || heading.level != 1 {
		t.Fatalf("h1 must parse as level-1 heading, got %T", elements[1])
	}
	if _, ok := elements[3].(mdTable); !ok {
		t.Fatalf("pipe table must parse, got %T", elements[3])
	}
	bullet, ok := elements[4].(mdBullet)
	if !ok || len(bullet.runs) != 2 || !bullet.runs[1].bold {
		t.Fatalf("bullet with bold run must parse, got %T %+v", elements[4], bullet.runs)
	}
	if _, ok := elements[5].(mdBullet); !ok {
		t.Fatalf("second bullet must parse, got %T", elements[5])
	}
	if _, ok := elements[6].(mdOrdered); !ok {
		t.Fatalf("ordered list must parse, got %T", elements[6])
	}
	if _, ok := elements[7].(mdDivider); !ok {
		t.Fatalf("divider must parse, got %T", elements[7])
	}
	if _, ok := elements[8].(mdParagraph); !ok {
		t.Fatalf("plain line must parse as paragraph, got %T", elements[8])
	}
}

func TestParseMarkdownTableSeparatorRowDropped(t *testing.T) {
	elements := elementsOf(t, "| 维度 | 得分 |\n| --- | --- |\n| 交付完成 | 80 |\n")
	table, ok := elements[0].(mdTable)
	if !ok {
		t.Fatalf("table must parse, got %T", elements[0])
	}
	if len(table.rows) != 2 {
		t.Fatalf("separator row must be dropped, rows = %v", table.rows)
	}
	if table.rows[1][0] != "交付完成" || table.rows[1][1] != "80" {
		t.Fatalf("cell text must survive parsing, rows = %v", table.rows)
	}
}

func TestParseInlineRunsStyles(t *testing.T) {
	runs := parseInlineRuns("完成 **登录改造** 与 `越权` 修复，见 [任务](https://pm.example.com/t/1)")
	if len(runs) != 6 {
		t.Fatalf("expected 6 runs, got %d: %+v", len(runs), runs)
	}
	if runs[0].text != "完成 " || runs[0].bold {
		t.Fatalf("leading plain run wrong: %+v", runs[0])
	}
	if runs[1].text != "登录改造" || !runs[1].bold {
		t.Fatalf("bold run wrong: %+v", runs[1])
	}
	if runs[3].text != "越权" || !runs[3].code {
		t.Fatalf("code run wrong: %+v", runs[3])
	}
	if runs[5].text != "任务" || runs[5].link != "https://pm.example.com/t/1" {
		t.Fatalf("link run wrong: %+v", runs[5])
	}
}

func TestParseMarkdownUnknownContentDegradesToParagraphs(t *testing.T) {
	elements := elementsOf(t, "随意的一行\n\n![图片](https://x/y.png)\n")
	for _, element := range elements {
		if _, ok := element.(mdParagraph); !ok {
			t.Fatalf("unrecognized content must degrade to paragraphs, got %T", element)
		}
	}
}

package quality

import "strings"

func StatusLabel(value string) string {
	key := strings.ToLower(strings.TrimSpace(value))
	labels := map[string]string{
		"triage": "待确认", "todo": "待处理", "in_progress": "处理中",
		"reopened": "重新打开", "review": "回归验证", "done": "已关闭", "canceled": "取消",
	}
	if label := labels[key]; label != "" {
		return label
	}
	return strings.TrimSpace(value)
}

func SeverityLabel(value string) string {
	key := strings.ToLower(strings.TrimSpace(value))
	labels := map[string]string{"p0": "致命", "p1": "严重", "p2": "一般", "p3": "轻微"}
	if label := labels[key]; label != "" {
		return label
	}
	return strings.TrimSpace(value)
}

func ResolutionReasonLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "as_designed":
		return "设计如此"
	case "external":
		return "外部原因"
	case "fixed":
		return "已解决"
	case "duplicate":
		return "重复Bug"
	case "cannot_reproduce":
		return "无法重现"
	case "postponed":
		return "延期处理"
	case "wont_fix":
		return "不予解决"
	case "not_a_bug":
		return "非缺陷"
	default:
		return strings.TrimSpace(value)
	}
}

package report

import (
	"strings"
	"time"
)

// Period is the resolved reporting window. Bounds are UTC instants derived
// from the viewer-provided X-Timezone location so a daily report covers the
// member's own business day.
type Period struct {
	Type       PeriodType `json:"type"`
	Start      time.Time  `json:"start"`
	End        time.Time  `json:"end"`
	AnchorDate string     `json:"anchor_date"`
}

const anchorDateLayout = "2006-01-02"

// ParseAnchorDate resolves the optional YYYY-MM-DD query value in the viewer
// location. An empty value anchors the period on today in that location.
func ParseAnchorDate(value string, location *time.Location, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if location == nil {
		location = time.UTC
	}
	if value == "" {
		local := now.In(location)
		return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location), nil
	}
	anchor, err := time.ParseInLocation(anchorDateLayout, value, location)
	if err != nil {
		return time.Time{}, ErrReportPeriodInvalid
	}
	return anchor, nil
}

// ResolvePeriod expands the anchor date into the [start, end) window for the
// requested cadence: a calendar day, the ISO week starting Monday, or the
// calendar month. 周报与月报只回顾已结束的完整周期：若解析出的窗口尚未结束
// （end 在未来，即锚点落在本周/本月），自动回退到上一个完整周期——本周还没
// 过完，无法生成周报。历史锚点不受影响；日报恒为锚点当天（晨间进展口径）。
// AnchorDate 归一为覆盖区间内的最后一天，作为报告的落款日期。
func ResolvePeriod(periodType PeriodType, anchor time.Time, location *time.Location, now time.Time) (Period, error) {
	if !periodType.Valid() {
		return Period{}, ErrReportPeriodInvalid
	}
	if location == nil {
		location = time.UTC
	}
	local := anchor.In(location)
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	period := Period{Type: periodType}
	switch periodType {
	case PeriodDaily:
		period.Start = dayStart
		period.End = dayStart.AddDate(0, 0, 1)
	case PeriodWeekly:
		offset := (int(dayStart.Weekday()) + 6) % 7
		period.Start = dayStart.AddDate(0, 0, -offset)
		period.End = period.Start.AddDate(0, 0, 7)
	case PeriodMonthly:
		period.Start = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
		period.End = period.Start.AddDate(0, 1, 0)
	}
	if periodType != PeriodDaily && period.End.After(now) {
		// 尚未结束的周/月回退一个周期，得到上一个完整窗口；月度用日历
		// 回退（AddDate 归一化月末差异），不能按固定长度减。
		if periodType == PeriodMonthly {
			period.Start = period.Start.AddDate(0, -1, 0)
			period.End = period.End.AddDate(0, -1, 0)
		} else {
			period.Start = period.Start.AddDate(0, 0, -7)
			period.End = period.End.AddDate(0, 0, -7)
		}
	}
	period.AnchorDate = period.End.AddDate(0, 0, -1).Format(anchorDateLayout)
	return period, nil
}

func withinPeriod(moment *time.Time, period Period) bool {
	if moment == nil {
		return false
	}
	value := moment.UTC()
	return !value.Before(period.Start.UTC()) && value.Before(period.End.UTC())
}

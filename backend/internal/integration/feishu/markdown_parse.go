package feishu

import (
	"regexp"
	"strings"
)

// The report body arrives as markdown (facts tables + AI narrative). This
// parser reduces it to the block elements the docx writer can render
// natively: headings, paragraphs, bullet/ordered lists, dividers, callouts
// (from quotes), and pipe tables. Anything unrecognized degrades to a plain
// paragraph so publishing never loses content.

type mdElement interface{ isMDElement() }

type mdHeading struct {
	level int // 1, 2, or 3
	runs  []*inlineRun
}

type mdParagraph struct{ runs []*inlineRun }

type mdBullet struct{ runs []*inlineRun }

type mdOrdered struct{ runs []*inlineRun }

type mdDivider struct{}

type mdCallout struct {
	lines [][]*inlineRun
}

type mdTable struct{ rows [][]string }

func (mdHeading) isMDElement()   {}
func (mdParagraph) isMDElement() {}
func (mdBullet) isMDElement()    {}
func (mdOrdered) isMDElement()   {}
func (mdDivider) isMDElement()   {}
func (mdCallout) isMDElement()   {}
func (mdTable) isMDElement()     {}

// inlineRun is one styled text fragment inside a block.
type inlineRun struct {
	text string
	bold bool
	code bool
	link string
}

var (
	headingPattern   = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	bulletPattern    = regexp.MustCompile(`^\s*[-*]\s+(.*)$`)
	orderedPattern   = regexp.MustCompile(`^\s*\d+[.、)]\s+(.*)$`)
	dividerPattern   = regexp.MustCompile(`^\s*-{3,}\s*$`)
	quotePattern     = regexp.MustCompile(`^\s*>\s?(.*)$`)
	tableRowPattern  = regexp.MustCompile(`^\s*\|(.+)\|\s*$`)
	tableCellPattern = regexp.MustCompile(`^\s*:?-{2,}:?\s*$`)
	inlinePattern    = regexp.MustCompile(`\*\*([^*]+)\*\*|` + "`" + `([^` + "`" + `]+)` + "`" + `|\[([^\]]+)\]\(([^)\s]+)\)`)
)

// parseMarkdownElements splits the body into renderable elements.
func parseMarkdownElements(content string) []mdElement {
	elements := []mdElement{}
	calloutLines := [][]*inlineRun{}
	tableRows := [][]string{}

	flushCallout := func() {
		if len(calloutLines) > 0 {
			elements = append(elements, mdCallout{lines: calloutLines})
			calloutLines = nil
		}
	}
	flushTable := func() {
		if len(tableRows) > 0 {
			elements = append(elements, mdTable{rows: tableRows})
			tableRows = nil
		}
	}
	flushAll := func() {
		flushCallout()
		flushTable()
	}

	for _, rawLine := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		line := strings.TrimRight(rawLine, " \t")
		switch {
		case strings.TrimSpace(line) == "":
			flushAll()
		case tableRowPattern.MatchString(line):
			flushCallout()
			row := parseTableRow(line)
			// The separator row under a header is dropped; it only marks
			// the previous row as the table header.
			if isTableSeparatorRow(row) {
				continue
			}
			tableRows = append(tableRows, row)
		case dividerPattern.MatchString(line):
			flushAll()
			elements = append(elements, mdDivider{})
		case quotePattern.MatchString(line):
			flushTable()
			match := quotePattern.FindStringSubmatch(line)
			calloutLines = append(calloutLines, parseInlineRuns(match[1]))
		case headingPattern.MatchString(line):
			flushAll()
			match := headingPattern.FindStringSubmatch(line)
			runs := parseInlineRuns(match[2])
			if len(runs) == 0 {
				continue
			}
			elements = append(elements, mdHeading{level: minInt(len(match[1]), 3), runs: runs})
		case bulletPattern.MatchString(line):
			flushAll()
			match := bulletPattern.FindStringSubmatch(line)
			elements = append(elements, mdBullet{runs: parseInlineRuns(match[1])})
		case orderedPattern.MatchString(line):
			flushAll()
			match := orderedPattern.FindStringSubmatch(line)
			elements = append(elements, mdOrdered{runs: parseInlineRuns(match[1])})
		default:
			flushAll()
			elements = append(elements, mdParagraph{runs: parseInlineRuns(line)})
		}
	}
	flushAll()
	return elements
}

func parseTableRow(line string) []string {
	match := tableRowPattern.FindStringSubmatch(line)
	cells := strings.Split(match[1], "|")
	row := make([]string, 0, len(cells))
	for _, cell := range cells {
		row = append(row, strings.TrimSpace(cell))
	}
	return row
}

func isTableSeparatorRow(row []string) bool {
	nonEmpty := 0
	for _, cell := range row {
		if cell == "" {
			continue
		}
		nonEmpty++
		if !tableCellPattern.MatchString(cell) {
			return false
		}
	}
	return nonEmpty > 0
}

// parseInlineRuns splits a line into styled runs: **bold**, `code`, and
// [label](url) links; everything else stays plain.
func parseInlineRuns(line string) []*inlineRun {
	runs := []*inlineRun{}
	cursor := 0
	appendPlain := func(end int) {
		if end > cursor {
			runs = append(runs, &inlineRun{text: line[cursor:end]})
		}
	}
	for _, match := range inlinePattern.FindAllStringSubmatchIndex(line, -1) {
		appendPlain(match[0])
		switch {
		case match[2] >= 0:
			runs = append(runs, &inlineRun{text: line[match[2]:match[3]], bold: true})
		case match[4] >= 0:
			runs = append(runs, &inlineRun{text: line[match[4]:match[5]], code: true})
		case match[6] >= 0:
			runs = append(runs, &inlineRun{text: line[match[6]:match[7]], link: line[match[8]:match[9]]})
		}
		cursor = match[1]
	}
	appendPlain(len(line))
	return runs
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

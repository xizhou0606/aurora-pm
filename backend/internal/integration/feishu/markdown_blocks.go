package feishu

import (
	"context"
	"fmt"

	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
)

// Feishu docx block types used by the report writer.
const (
	blockTypeText      = 2
	blockTypeHeading1  = 3
	blockTypeHeading2  = 4
	blockTypeHeading3  = 5
	blockTypeBullet    = 12
	blockTypeOrdered   = 13
	blockTypeCallout   = 19
	blockTypeDivider   = 22
	blockTypeTable     = 31
	blockTypeTableCell = 32
)

// calloutBackgroundYellow renders the meta callout on the light-yellow
// highlight background of the Feishu callout color palette.
const calloutBackgroundYellow = 3

// appendParsedBlocks is the fallback writer used when the markdown convert
// API is unavailable: it renders the hand-parsed markdown into native docx
// blocks. Tables and callouts degrade to plain text lines if their nested
// create is rejected, so publishing never loses content.
func (r *SDKReader) appendParsedBlocks(ctx context.Context, documentID string, content string) error {
	batch := make([]*larkdocx.Block, 0, maxBlocksPerRequest)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := r.createBlocks(ctx, documentID, batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}
	for _, element := range parseMarkdownElements(content) {
		switch value := element.(type) {
		case mdTable:
			if err := flush(); err != nil {
				return err
			}
			if err := r.createTableBlock(ctx, documentID, value); err != nil {
				batch = append(batch, tableFallbackBlocks(value)...)
				if len(batch) >= maxBlocksPerRequest {
					if err := flush(); err != nil {
						return err
					}
				}
				continue
			}
		case mdCallout:
			if err := flush(); err != nil {
				return err
			}
			if err := r.createCalloutBlock(ctx, documentID, value); err != nil {
				for _, runs := range value.lines {
					batch = append(batch, larkdocx.NewBlockBuilder().
						BlockType(blockTypeText).
						Text(textBlock(runs)).
						Build())
				}
				if len(batch) >= maxBlocksPerRequest {
					if err := flush(); err != nil {
						return err
					}
				}
				continue
			}
		case mdDivider:
			if err := flush(); err != nil {
				return err
			}
			batch = append(batch, larkdocx.NewBlockBuilder().
				BlockType(blockTypeDivider).
				Divider(larkdocx.NewDividerBuilder().Build()).
				Build())
		case mdHeading:
			batch = append(batch, headingBlock(value.level, value.runs))
		case mdBullet:
			batch = append(batch, larkdocx.NewBlockBuilder().
				BlockType(blockTypeBullet).
				Bullet(textBlock(value.runs)).
				Build())
		case mdOrdered:
			batch = append(batch, larkdocx.NewBlockBuilder().
				BlockType(blockTypeOrdered).
				Ordered(textBlock(value.runs)).
				Build())
		case mdParagraph:
			batch = append(batch, larkdocx.NewBlockBuilder().
				BlockType(blockTypeText).
				Text(textBlock(value.runs)).
				Build())
		}
		if len(batch) >= maxBlocksPerRequest {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}

// tableFallbackBlocks renders a rejected table as plain text rows so the
// content still lands in the document.
func tableFallbackBlocks(table mdTable) []*larkdocx.Block {
	blocks := make([]*larkdocx.Block, 0, len(table.rows))
	for _, row := range table.rows {
		cells := make([]*inlineRun, 0, len(row))
		for columnIndex, cell := range row {
			if columnIndex > 0 {
				cells = append(cells, &inlineRun{text: " ｜ "})
			}
			cells = append(cells, parseInlineRuns(cell)...)
		}
		blocks = append(blocks, larkdocx.NewBlockBuilder().
			BlockType(blockTypeText).
			Text(textBlock(cells)).
			Build())
	}
	return blocks
}

func headingBlock(level int, runs []*inlineRun) *larkdocx.Block {
	text := textBlock(runs)
	builder := larkdocx.NewBlockBuilder()
	switch level {
	case 1:
		return builder.BlockType(blockTypeHeading1).Heading1(text).Build()
	case 2:
		return builder.BlockType(blockTypeHeading2).Heading2(text).Build()
	default:
		return builder.BlockType(blockTypeHeading3).Heading3(text).Build()
	}
}

// textBlock renders styled inline runs as one text body. Empty runs degrade
// to a single space so Feishu never rejects an empty element list.
func textBlock(runs []*inlineRun) *larkdocx.Text {
	elements := make([]*larkdocx.TextElement, 0, len(runs))
	for _, run := range runs {
		content := run.text
		if content == "" {
			continue
		}
		styleBuilder := larkdocx.NewTextElementStyleBuilder()
		styled := false
		if run.bold {
			styleBuilder = styleBuilder.Bold(true)
			styled = true
		}
		if run.code {
			styleBuilder = styleBuilder.InlineCode(true)
			styled = true
		}
		if run.link != "" {
			styleBuilder = styleBuilder.Link(larkdocx.NewLinkBuilder().Url(run.link).Build())
			styled = true
		}
		runBuilder := larkdocx.NewTextRunBuilder().Content(content)
		if styled {
			runBuilder = runBuilder.TextElementStyle(styleBuilder.Build())
		}
		elements = append(elements, larkdocx.NewTextElementBuilder().
			TextRun(runBuilder.Build()).
			Build())
	}
	if len(elements) == 0 {
		elements = append(elements, larkdocx.NewTextElementBuilder().
			TextRun(larkdocx.NewTextRunBuilder().Content(" ").Build()).
			Build())
	}
	return larkdocx.NewTextBuilder().Elements(elements).Build()
}

// createTableBlock writes one table with all cell text through the
// nested-block API: the table block plus every table_cell and its text
// child travel as a single descendants tree.
func (r *SDKReader) createTableBlock(
	ctx context.Context,
	documentID string,
	table mdTable,
) error {
	if len(table.rows) == 0 {
		return nil
	}
	columns := len(table.rows[0])
	rowSize := len(table.rows)
	descendants := []*larkdocx.Block{}
	tableChildren := make([]string, 0, rowSize*columns)
	tableTempID := "pm_report_table"
	for rowIndex, row := range table.rows {
		for columnIndex := 0; columnIndex < columns; columnIndex++ {
			cellTempID := fmt.Sprintf("pm_report_cell_%d_%d", rowIndex, columnIndex)
			textTempID := fmt.Sprintf("pm_report_cell_text_%d_%d", rowIndex, columnIndex)
			cellText := ""
			if columnIndex < len(row) {
				cellText = row[columnIndex]
			}
			text := textBlock(inlineRunsForCell(rowIndex, cellText))
			textBlockRef := larkdocx.NewBlockBuilder().
				BlockType(blockTypeText).
				BlockId(textTempID).
				Text(text).
				Build()
			cellBlock := larkdocx.NewBlockBuilder().
				BlockType(blockTypeTableCell).
				BlockId(cellTempID).
				Children([]string{textTempID}).
				Build()
			tableChildren = append(tableChildren, cellTempID)
			descendants = append(descendants, cellBlock, textBlockRef)
		}
	}
	tableBlock := larkdocx.NewBlockBuilder().
		BlockType(blockTypeTable).
		BlockId(tableTempID).
		Children(tableChildren).
		Table(larkdocx.NewTableBuilder().Property(larkdocx.NewTablePropertyBuilder().
			RowSize(rowSize).
			ColumnSize(columns).
			ColumnWidth(tableColumnWidths(columns)).
			Build()).Build()).
		Build()
	return r.createDescendants(ctx, documentID, []string{tableTempID},
		append([]*larkdocx.Block{tableBlock}, descendants...))
}

// inlineRunsForCell renders header-row cells bold so tables read properly.
func inlineRunsForCell(rowIndex int, content string) []*inlineRun {
	if content == "" {
		content = " "
	}
	if rowIndex == 0 {
		return []*inlineRun{{text: content, bold: true}}
	}
	return parseInlineRuns(content)
}

// tableColumnWidths distributes the ~730px Feishu page content width across
// the columns; the API minimum per column is 50px.
func tableColumnWidths(columns int) []int {
	if columns <= 0 {
		return nil
	}
	const totalWidth = 730
	const minWidth = 50
	width := totalWidth / columns
	if width < minWidth {
		width = minWidth
	}
	widths := make([]int, columns)
	used := 0
	for index := range widths {
		widths[index] = width
		used += width
	}
	widths[columns-1] += totalWidth - used
	return widths
}

// createCalloutBlock writes one highlighted callout block whose children are
// the quoted lines.
func (r *SDKReader) createCalloutBlock(
	ctx context.Context,
	documentID string,
	callout mdCallout,
) error {
	calloutTempID := "pm_report_callout"
	descendants := []*larkdocx.Block{}
	children := make([]string, 0, len(callout.lines))
	for index, runs := range callout.lines {
		textTempID := fmt.Sprintf("pm_report_callout_text_%d", index)
		textBlockRef := larkdocx.NewBlockBuilder().
			BlockType(blockTypeText).
			BlockId(textTempID).
			Text(textBlock(runs)).
			Build()
		children = append(children, textTempID)
		descendants = append(descendants, textBlockRef)
	}
	calloutBlock := larkdocx.NewBlockBuilder().
		BlockType(blockTypeCallout).
		BlockId(calloutTempID).
		Children(children).
		Callout(larkdocx.NewCalloutBuilder().BackgroundColor(calloutBackgroundYellow).Build()).
		Build()
	return r.createDescendants(ctx, documentID, []string{calloutTempID},
		append([]*larkdocx.Block{calloutBlock}, descendants...))
}

func (r *SDKReader) createDescendants(
	ctx context.Context,
	documentID string,
	childrenIDs []string,
	descendants []*larkdocx.Block,
) error {
	// Index -1 appends at the end of the block list, matching the reference
	// implementation of the community Feishu plugin.
	appendAtEnd := -1
	response, err := r.client.Docx.V1.DocumentBlockDescendant.Create(ctx,
		larkdocx.NewCreateDocumentBlockDescendantReqBuilder().
			DocumentId(documentID).
			BlockId(documentID).
			Body(larkdocx.NewCreateDocumentBlockDescendantReqBodyBuilder().
				ChildrenId(childrenIDs).
				Descendants(descendants).
				Index(appendAtEnd).
				Build()).
			Build())
	if err != nil {
		return fmt.Errorf("write Feishu document blocks: %w", err)
	}
	if response != nil && response.Code != 0 {
		return fmt.Errorf("write Feishu document blocks: code %d: %s", response.Code, response.Msg)
	}
	return nil
}

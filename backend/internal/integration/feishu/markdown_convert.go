package feishu

import (
	"context"
	"fmt"

	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
)

// The report body is markdown, so block generation delegates to Feishu's
// official markdown conversion API and inserts the converted tree through
// the nested-block API — the same architecture as the community Feishu
// plugin, with server-authoritative block shapes (headings, tables with
// cells, quotes, bold/links). The hand-rolled parser in markdown_parse.go /
// markdown_blocks.go only serves as the fallback when conversion is
// unavailable (e.g. missing convert scope).

// maxDescendantsPerRequest keeps each nested-block insert under the Feishu
// API limit of 1000 blocks per request.
const maxDescendantsPerRequest = 900

type convertedBlocks struct {
	firstLevelIDs []string
	blocks        []*larkdocx.Block
}

// convertMarkdownBlocks converts the markdown body into a Feishu block tree.
func (r *SDKReader) convertMarkdownBlocks(ctx context.Context, content string) (convertedBlocks, error) {
	response, err := r.client.Docx.V1.Document.Convert(ctx,
		larkdocx.NewConvertDocumentReqBuilder().
			Body(larkdocx.NewConvertDocumentReqBodyBuilder().
				ContentType("markdown").
				Content(content).
				Build()).
			Build())
	if err != nil {
		return convertedBlocks{}, fmt.Errorf("convert Feishu document content: %w", err)
	}
	if response != nil && response.Code != 0 {
		return convertedBlocks{}, fmt.Errorf("convert Feishu document content: code %d: %s", response.Code, response.Msg)
	}
	if response == nil || response.Data == nil {
		return convertedBlocks{}, fmt.Errorf("convert Feishu document content: empty response")
	}
	return convertedBlocks{
		firstLevelIDs: response.Data.FirstLevelBlockIds,
		blocks:        response.Data.Blocks,
	}, nil
}

// insertConvertedBlocks writes the converted tree under the document root in
// batches that each stay inside the per-request block limit while keeping
// every first-level block (and its subtree) in exactly one request.
func (r *SDKReader) insertConvertedBlocks(
	ctx context.Context,
	documentID string,
	converted convertedBlocks,
) error {
	blockMap := make(map[string]*larkdocx.Block, len(converted.blocks))
	for _, block := range cleanConvertedBlocks(converted.blocks) {
		if block.BlockId == nil {
			continue
		}
		blockMap[*block.BlockId] = block
	}
	used := make(map[string]bool, len(converted.blocks))
	chunkIDs := make([]string, 0, 32)
	chunkBlocks := make([]*larkdocx.Block, 0, maxDescendantsPerRequest)
	flushChunk := func() error {
		if len(chunkIDs) == 0 {
			return nil
		}
		if err := r.createDescendants(ctx, documentID, chunkIDs, chunkBlocks); err != nil {
			return err
		}
		chunkIDs = make([]string, 0, 32)
		chunkBlocks = chunkBlocks[:0]
		return nil
	}
	for _, id := range converted.firstLevelIDs {
		tree := collectConvertedTree(id, blockMap, used)
		if len(chunkIDs) > 0 && len(chunkBlocks)+len(tree) > maxDescendantsPerRequest {
			if err := flushChunk(); err != nil {
				return err
			}
		}
		chunkIDs = append(chunkIDs, id)
		chunkBlocks = append(chunkBlocks, tree...)
		for _, block := range tree {
			used[*block.BlockId] = true
		}
	}
	return flushChunk()
}

// collectConvertedTree walks a first-level block's subtree depth-first so
// parents precede children in the descendants list.
func collectConvertedTree(
	rootID string,
	blockMap map[string]*larkdocx.Block,
	used map[string]bool,
) []*larkdocx.Block {
	tree := []*larkdocx.Block{}
	var visit func(blockID string)
	visit = func(blockID string) {
		if used[blockID] {
			return
		}
		block := blockMap[blockID]
		if block == nil {
			return
		}
		used[blockID] = true
		tree = append(tree, block)
		for _, childID := range block.Children {
			visit(childID)
		}
	}
	visit(rootID)
	return tree
}

// cleanConvertedBlocks drops fields the nested-block create rejects and
// rebuilds table properties with explicit column widths. Converted tables
// carry a cells array of pre-allocated ids which the create API must not
// receive; the cells themselves arrive as table_cell children instead.
func cleanConvertedBlocks(blocks []*larkdocx.Block) []*larkdocx.Block {
	cleaned := make([]*larkdocx.Block, 0, len(blocks))
	for _, block := range blocks {
		block.ParentId = nil
		block.CommentIds = nil
		if table := block.Table; table != nil {
			property := table.Property
			if property != nil {
				columns := 0
				if property.ColumnSize != nil {
					columns = *property.ColumnSize
				}
				widths := property.ColumnWidth
				if len(widths) == 0 {
					widths = tableColumnWidths(columns)
				}
				for index, width := range widths {
					if width < 50 {
						widths[index] = 50
					}
				}
				block.Table = &larkdocx.Table{
					Property: &larkdocx.TableProperty{
						RowSize:     property.RowSize,
						ColumnSize:  property.ColumnSize,
						ColumnWidth: widths,
					},
				}
			} else {
				block.Table = &larkdocx.Table{}
			}
		}
		cleaned = append(cleaned, block)
	}
	return cleaned
}

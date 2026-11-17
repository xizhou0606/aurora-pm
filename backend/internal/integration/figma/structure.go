package figma

import "encoding/json"

type figmaBounds struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type figmaNode struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Type                string          `json:"type"`
	Visible             *bool           `json:"visible,omitempty"`
	AbsoluteBoundingBox *figmaBounds    `json:"absoluteBoundingBox,omitempty"`
	Fills               json.RawMessage `json:"fills,omitempty"`
	Strokes             json.RawMessage `json:"strokes,omitempty"`
	Effects             json.RawMessage `json:"effects,omitempty"`
	Style               json.RawMessage `json:"style,omitempty"`
	LayoutMode          *string         `json:"layoutMode,omitempty"`
	ItemSpacing         *float64        `json:"itemSpacing,omitempty"`
	PaddingTop          *float64        `json:"paddingTop,omitempty"`
	PaddingRight        *float64        `json:"paddingRight,omitempty"`
	PaddingBottom       *float64        `json:"paddingBottom,omitempty"`
	PaddingLeft         *float64        `json:"paddingLeft,omitempty"`
	CornerRadius        *float64        `json:"cornerRadius,omitempty"`
	Characters          *string         `json:"characters,omitempty"`
	Children            []figmaNode     `json:"children,omitempty"`
}

type sanitizedFigmaLayout struct {
	Mode         *string    `json:"mode,omitempty"`
	Gap          *float64   `json:"gap,omitempty"`
	Padding      []*float64 `json:"padding,omitempty"`
	CornerRadius *float64   `json:"cornerRadius,omitempty"`
}

type sanitizedFigmaNode struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	Type              string               `json:"type"`
	Visible           bool                 `json:"visible"`
	Bounds            *figmaBounds         `json:"bounds,omitempty"`
	Fills             json.RawMessage      `json:"fills,omitempty"`
	Strokes           json.RawMessage      `json:"strokes,omitempty"`
	Effects           json.RawMessage      `json:"effects,omitempty"`
	Style             json.RawMessage      `json:"style,omitempty"`
	Layout            sanitizedFigmaLayout `json:"layout"`
	Text              *string              `json:"text,omitempty"`
	Children          []sanitizedFigmaNode `json:"children,omitempty"`
	ChildrenTruncated bool                 `json:"childrenTruncated,omitempty"`
	Truncated         bool                 `json:"truncated,omitempty"`
}

func sanitizeFigmaNode(node figmaNode, depth int) sanitizedFigmaNode {
	visible := node.Visible == nil || *node.Visible
	result := sanitizedFigmaNode{ID: node.ID, Name: node.Name, Type: node.Type, Visible: visible, Bounds: node.AbsoluteBoundingBox,
		Fills: node.Fills, Strokes: node.Strokes, Effects: node.Effects, Style: node.Style,
		Layout: sanitizedFigmaLayout{Mode: node.LayoutMode, Gap: node.ItemSpacing, Padding: []*float64{node.PaddingTop, node.PaddingRight, node.PaddingBottom, node.PaddingLeft}, CornerRadius: node.CornerRadius}}
	if node.Characters != nil {
		text := []rune(*node.Characters)
		if len(text) > 500 {
			text = text[:500]
		}
		value := string(text)
		result.Text = &value
	}
	if depth >= 12 {
		result.Truncated = len(node.Children) > 0
		return result
	}
	children := node.Children
	if len(children) > 200 {
		children = children[:200]
		result.ChildrenTruncated = true
	}
	result.Children = make([]sanitizedFigmaNode, 0, len(children))
	for _, child := range children {
		result.Children = append(result.Children, sanitizeFigmaNode(child, depth+1))
	}
	return result
}

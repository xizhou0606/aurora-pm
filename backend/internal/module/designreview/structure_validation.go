package designreview

import (
	"bytes"
	"encoding/json"
	"strings"
)

type structureTopbar struct {
	Enabled         bool   `json:"enabled"`
	Platform        string `json:"platform,omitempty"`
	Height          int    `json:"height,omitempty"`
	Time            string `json:"time,omitempty"`
	BackgroundColor string `json:"background_color,omitempty"`
	IconTheme       string `json:"icon_theme,omitempty"`
}

func validImplementationStructureEnvelope(structure implementationStructure) bool {
	if structure.FinalViewport.Width <= 0 || structure.FinalViewport.Height <= 0 ||
		structure.FinalViewport.Width > 7680 || structure.FinalViewport.Height > 7680 ||
		structure.ContentViewport.Width != structure.FinalViewport.Width {
		return false
	}
	if structure.Source == "mobile" {
		if strings.TrimSpace(structure.ActualURL) != "" || structure.Target == nil || !validMobileStructureTarget(*structure.Target, structure.FinalViewport) ||
			!bytes.Equal(bytes.TrimSpace(structure.SyntheticMobileTopbar), []byte("null")) || structure.ContentViewport != structure.FinalViewport {
			return false
		}
		return true
	}
	if structure.Source != "web" || structure.Target != nil || strings.TrimSpace(structure.ActualURL) != "" && !validHTTPURL(structure.ActualURL) {
		return false
	}
	var topbar structureTopbar
	if decodeStrictJSON(structure.SyntheticMobileTopbar, &topbar) != nil {
		return bytes.Equal(bytes.TrimSpace(structure.SyntheticMobileTopbar), []byte("null")) && structure.ContentViewport == structure.FinalViewport
	}
	if !topbar.Enabled {
		return topbar == (structureTopbar{}) && structure.ContentViewport == structure.FinalViewport
	}
	return topbar.Platform == "ios" && topbar.Height == 44 && len(topbar.Time) <= 16 && validHexColor(topbar.BackgroundColor) &&
		(topbar.IconTheme == "light" || topbar.IconTheme == "dark") && structure.ContentViewport.Height+topbar.Height == structure.FinalViewport.Height
}

func validMobileStructureTarget(target DesignReviewTarget, viewport structureViewport) bool {
	if target.Kind != "mobile" || !target.Valid() || len(target.AppID) > 300 || len(target.AppName) > 300 || len(target.AppVersion) > 120 ||
		len(target.Collector) > 40 || len(target.Mobilewright) > 40 || !jsonObject(target.Device) {
		return false
	}
	var screen struct {
		Width  int     `json:"width"`
		Height int     `json:"height"`
		Scale  float64 `json:"scale"`
	}
	if json.Unmarshal(target.Screen, &screen) != nil || screen.Width != viewport.Width || screen.Height != viewport.Height || screen.Scale <= 0 || screen.Scale > 8 {
		return false
	}
	return true
}

func jsonObject(raw json.RawMessage) bool {
	var value map[string]json.RawMessage
	return len(raw) > 0 && json.Unmarshal(raw, &value) == nil && value != nil
}

func validWebStructureElement(element webStructureElement) bool {
	if strings.TrimSpace(element.Tag) == "" || len(element.Tag) > 80 || tooLong(element.Role, 80) || tooLong(element.Text, 500) ||
		len(element.Styles) > 100 || len(element.AllowedOperations) > 3 {
		return false
	}
	for key, value := range element.Styles {
		if strings.TrimSpace(key) == "" || len(key) > 120 || len(value) > 500 {
			return false
		}
	}
	if element.ControlState != nil && (tooLong(element.ControlState.AriaChecked, 80) || tooLong(element.ControlState.AriaPressed, 80) ||
		tooLong(element.ControlState.AriaSelected, 80) || tooLong(element.ControlState.DataState, 80)) {
		return false
	}
	return true
}

func validNativeStructureElement(element nativeStructureElement) bool {
	return strings.TrimSpace(element.NativeType) != "" && len(element.NativeType) <= 120 &&
		!tooLong(element.Role, 80) && !tooLong(element.Label, 500) && !tooLong(element.Text, 500) && !tooLong(element.Value, 500) &&
		!tooLong(element.Identifier, 300) && !tooLong(element.ResourceID, 300)
}

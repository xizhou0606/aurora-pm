package document

import (
	"net/url"
	"strings"
)

func analysisMarkedText(n analysisNode) string {
	text := n.Text
	for _, mark := range n.Marks {
		switch mark.Type {
		case "bold":
			text = "**" + text + "**"
		case "italic":
			text = "*" + text + "*"
		case "strike":
			text = "~~" + text + "~~"
		case "code":
			fence := "`"
			for strings.Contains(text, fence) {
				fence += "`"
			}
			text = fence + " " + text + " " + fence
		case "link":
			href, _ := mark.Attrs["href"].(string)
			parsed, err := url.Parse(href)
			if err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil {
				// Query strings and fragments can carry credentials; never send them to a model.
				parsed.RawQuery = ""
				parsed.ForceQuery = false
				parsed.Fragment = ""
				parsed.RawFragment = ""
				text += " (" + parsed.String() + "; linked content not expanded)"
			} else {
				text += " [link not expanded]"
			}
		}
	}
	return text
}

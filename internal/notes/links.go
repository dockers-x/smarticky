package notes

import (
	"regexp"
	"strings"
	"unicode"
)

const wikiLinkType = "wiki"

var wikiLinkPattern = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)

type ParsedLink struct {
	TargetRef       string
	TargetRefNorm   string
	DisplayText     string
	LinkType        string
	OccurrenceCount int
}

func ParseWikiLinks(content string) []ParsedLink {
	matches := wikiLinkPattern.FindAllStringSubmatch(content, -1)
	links := make([]ParsedLink, 0, len(matches))
	byNorm := make(map[string]int, len(matches))

	for _, match := range matches {
		targetRef, displayText := splitWikiLink(match[1])
		norm := normalizeLinkRef(targetRef)
		if norm == "" {
			continue
		}

		if idx, ok := byNorm[norm]; ok {
			links[idx].OccurrenceCount++
			continue
		}

		byNorm[norm] = len(links)
		links = append(links, ParsedLink{
			TargetRef:       targetRef,
			TargetRefNorm:   norm,
			DisplayText:     displayText,
			LinkType:        wikiLinkType,
			OccurrenceCount: 1,
		})
	}

	return links
}

func splitWikiLink(raw string) (string, string) {
	parts := strings.SplitN(raw, "|", 2)
	targetRef := strings.TrimSpace(parts[0])
	displayText := targetRef
	if len(parts) == 2 {
		displayText = strings.TrimSpace(parts[1])
		if displayText == "" {
			displayText = targetRef
		}
	}
	return targetRef, displayText
}

func normalizeLinkRef(value string) string {
	fields := strings.FieldsFunc(strings.TrimSpace(value), unicode.IsSpace)
	return strings.ToLower(strings.Join(fields, " "))
}

package models

import (
	"fmt"
	"regexp"
	"strings"
)

// WikiPage represents a single documentation page.
type WikiPage struct {
	Title   string
	Slug    string
	Level   string // Beginner / Intermediate / Advanced
	Section string
	Group   string // optional
}

// Wiki is the full catalog of documentation pages.
type Wiki struct {
	Pages []WikiPage
}

// FormatCatalog renders the catalog as markdown navigation, marking currentSlug.
func (w *Wiki) FormatCatalog(currentSlug string) string {
	var lines []string
	curSection := ""
	curGroup := ""

	for _, p := range w.Pages {
		if p.Section != curSection {
			curSection = p.Section
			curGroup = ""
			lines = append(lines, fmt.Sprintf("- **%s**", p.Section))
		}
		if p.Group != "" && p.Group != curGroup {
			curGroup = p.Group
			lines = append(lines, fmt.Sprintf("  - *%s*", p.Group))
		}
		indent := "  "
		if p.Group != "" {
			indent = "    "
		}
		marker := ""
		if p.Slug == currentSlug {
			marker = " [You are currently here]"
		}
		lines = append(lines, fmt.Sprintf("%s- [%s](%s)%s", indent, p.Title, p.Slug, marker))
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Slug generation
// ---------------------------------------------------------------------------

var (
	reNonAlnum     = regexp.MustCompile(`[^a-z0-9]+`)
	reLeadTrailDash = regexp.MustCompile(`^-+|-+$`)
)

// slugify converts a title to a URL-safe ASCII slug.
// Non-ASCII characters (e.g. Chinese) are replaced, resulting in "" which
// triggers the numeric fallback in MakeSlug — matching Python's behavior
// with allow_unicode=False.
func slugify(title string) string {
	result := strings.ToLower(title)
	result = reNonAlnum.ReplaceAllString(result, "-")
	result = reLeadTrailDash.ReplaceAllString(result, "")
	return result
}

// MakeSlug creates a numbered slug like "1-overview" or "2-2" (for non-ASCII titles).
func MakeSlug(counter int, title string) string {
	part := slugify(title)
	if part == "" {
		part = fmt.Sprintf("%d", counter)
	}
	return fmt.Sprintf("%d-%s", counter, part)
}

// ---------------------------------------------------------------------------
// Catalog XML parser
// ---------------------------------------------------------------------------

var (
	reSectionBlock = regexp.MustCompile(`(?s)<section>(.*?)</section>`)
	reTopicFull    = regexp.MustCompile(`(?s)<topic[^>]*level=["']?([^"'>\s]+)["']?[^>]*>(.*?)</topic>`)
	reTopicLenient = regexp.MustCompile(`(?s)<topic[^>]*>(.*?)</topic>`)
	reLevelAttr    = regexp.MustCompile(`level=["']?([^"'>\s]+)`)
)

// ParseCatalogXML parses the <section>/<topic>/<group> XML produced by the catalog agent.
func ParseCatalogXML(xmlText string) *Wiki {
	wiki := &Wiki{}
	counter := 0

	for _, secMatch := range reSectionBlock.FindAllStringSubmatch(xmlText, -1) {
		secContent := secMatch[1]

		// Section name: text before first <topic or <group
		firstTagPos := len(secContent)
		for _, tag := range []string{"<topic", "<group"} {
			if p := strings.Index(secContent, tag); p != -1 && p < firstTagPos {
				firstTagPos = p
			}
		}
		sectionName := strings.TrimSpace(secContent[:firstTagPos])
		if sectionName == "" {
			continue
		}

		remaining := secContent[firstTagPos:]
		pos := 0
		for pos < len(remaining) {
			gStart := strings.Index(remaining[pos:], "<group>")
			tStart := strings.Index(remaining[pos:], "<topic")
			if gStart != -1 {
				gStart += pos
			}
			if tStart != -1 {
				tStart += pos
			}

			if gStart == -1 && tStart == -1 {
				break
			}

			if gStart != -1 && (tStart == -1 || gStart < tStart) {
				// Group block
				gEnd := strings.Index(remaining[gStart:], "</group>")
				if gEnd == -1 {
					break
				}
				gEnd += gStart
				groupContent := remaining[gStart+7 : gEnd]
				// Group name: text before first <topic
				grpName := ""
				if grpTopicPos := strings.Index(groupContent, "<topic"); grpTopicPos != -1 {
					grpName = strings.TrimSpace(groupContent[:grpTopicPos])
				}
				// Topics inside group
				for _, tm := range reTopicFull.FindAllStringSubmatch(groupContent, -1) {
					counter++
					level := strings.TrimSpace(tm[1])
					title := strings.TrimSpace(tm[2])
					if title == "" {
						continue
					}
					wiki.Pages = append(wiki.Pages, WikiPage{
						Title:   title,
						Slug:    MakeSlug(counter, title),
						Level:   level,
						Section: sectionName,
						Group:   grpName,
					})
				}
				pos = gEnd + 8
			} else {
				// Standalone topic
				tEnd := strings.Index(remaining[tStart:], "</topic>")
				if tEnd == -1 {
					break
				}
				tEnd += tStart + len("</topic>")
				topicStr := remaining[tStart:tEnd]

				var level, title string
				if m := reTopicFull.FindStringSubmatch(topicStr); m != nil {
					level = strings.TrimSpace(m[1])
					title = strings.TrimSpace(m[2])
				} else if m2 := reTopicLenient.FindStringSubmatch(topicStr); m2 != nil {
					title = strings.TrimSpace(m2[1])
					if lm := reLevelAttr.FindStringSubmatch(topicStr); lm != nil {
						level = strings.TrimSpace(lm[1])
					} else {
						level = "Beginner"
					}
				}
				if title != "" {
					counter++
					wiki.Pages = append(wiki.Pages, WikiPage{
						Title:   title,
						Slug:    MakeSlug(counter, title),
						Level:   level,
						Section: sectionName,
					})
				}
				pos = tEnd
			}
		}
	}
	return wiki
}

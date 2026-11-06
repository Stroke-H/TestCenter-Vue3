package services

import (
	"fmt"
	"strings"
)

const dramaRuleCardDetailLimit = 30

// Same counting unit as the existing category headings: one drama per category.
// A drama with both a grouping and a copyright error contributes two entries.
func dramaRuleCardCount(groups, classifications, languages, copyrights []dramaFailureSummary) int {
	return len(groups) + len(classifications) + len(languages) + len(copyrights)
}

func dramaRuleCardSummary(groups, classifications, languages, copyrights []dramaFailureSummary) string {
	return fmt.Sprintf("四类规则异常共 %d 条分类明细，超过 %d 条，以下仅展示摘要；完整错误内容请打开分页明细。\n分组异常（%d 部）\n分类异常（%d 部）\n语种异常（%d 部）\n版权异常（%d 部）", dramaRuleCardCount(groups, classifications, languages, copyrights), dramaRuleCardDetailLimit, len(groups), len(classifications), len(languages), len(copyrights))
}

// Preserve oversized playback evidence too: split text into budgeted divs,
// never discard its tail. Leave room for page headers and repeated headings.
func splitScheduledCardTextElements(card map[string]any, elements []map[string]any, budget int) []map[string]any {
	result := []map[string]any{}
	for _, element := range elements {
		text, ok := element["text"].(map[string]any)
		if !ok {
			result = append(result, element)
			continue
		}
		content, ok := text["content"].(string)
		if !ok {
			result = append(result, element)
			continue
		}
		clone := func(content string) map[string]any {
			e, t := map[string]any{}, map[string]any{}
			for k, v := range element {
				e[k] = v
			}
			for k, v := range text {
				t[k] = v
			}
			t["content"] = content
			e["text"] = t
			return e
		}
		fits := func(content string) bool {
			probe := map[string]any{}
			for k, v := range card {
				probe[k] = v
			}
			probe["elements"] = []map[string]any{clone(content)}
			return scheduledCardEnvelopeSize(probe) <= budget
		}
		if fits(content) {
			result = append(result, element)
			continue
		}
		runes := []rune(content)
		for len(runes) > 0 {
			low, high := 0, len(runes)
			for low < high {
				mid := (low + high + 1) / 2
				if fits(string(runes[:mid])) {
					low = mid
				} else {
					high = mid - 1
				}
			}
			if low == 0 {
				result = append(result, clone(string(runes)))
				break
			}
			piece := string(runes[:low])
			if newline := strings.LastIndex(piece, "\n"); newline > len(piece)/2 {
				low = len([]rune(piece[:newline+1]))
				piece = string(runes[:low])
			}
			// Do not cut a normal-sized management link in the middle.
			if bracket := strings.LastIndex(piece, "["); bracket > 0 && strings.Contains(piece[bracket:], "](") && !strings.Contains(piece[bracket:], ")") {
				low = len([]rune(piece[:bracket]))
				piece = string(runes[:low])
			}
			result = append(result, clone(piece))
			runes = runes[low:]
		}
	}
	return result
}

// Package converter handles the conversion of v2fly domain list format to ruleset formats.
package converter

import (
	"fmt"
	"strings"
)

// Parse converts upstream content into parsed items with filter support.
func (c *Converter) Parse(upstreamContent string, filter string) ([]Item, error) {
	lines := strings.Split(upstreamContent, "\n")
	var items []Item

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#") {
			items = append(items, Item{
				Kind:    ItemComment,
				Comment: line,
			})
			continue
		}

		switch {
		case strings.HasPrefix(line, "domain:"):
			if rule, ok := parseRuleLine(line, "domain:", RuleDomainSuffix, filter); ok {
				items = append(items, Item{Kind: ItemRule, Rule: &rule})
			}
		case strings.HasPrefix(line, "full:"):
			if rule, ok := parseRuleLine(line, "full:", RuleDomain, filter); ok {
				items = append(items, Item{Kind: ItemRule, Rule: &rule})
			}
		case strings.HasPrefix(line, "keyword:"):
			if rule, ok := parseRuleLine(line, "keyword:", RuleDomainKeyword, filter); ok {
				items = append(items, Item{Kind: ItemRule, Rule: &rule})
			}
		case strings.HasPrefix(line, "regexp:"):
			if rule, ok := parseRuleLine(line, "regexp:", RuleDomainRegex, filter); ok {
				items = append(items, Item{Kind: ItemRule, Rule: &rule})
			}
		case strings.HasPrefix(line, "include:"):
			subItems, err := c.parseInclude(line, filter)
			if err != nil {
				return nil, err
			}
			items = append(items, subItems...)
		default:
			if rule, ok := parseRuleLine(line, "", RuleDomainSuffix, filter); ok {
				items = append(items, Item{Kind: ItemRule, Rule: &rule})
			}
		}
	}

	return items, nil
}

func parseRuleLine(line, fromPrefix string, kind RuleKind, filter string) (Rule, bool) {
	parts := strings.SplitN(line, " ", 2)
	value := parts[0]
	if fromPrefix != "" {
		value = strings.TrimPrefix(value, fromPrefix)
	}
	rest := ""
	if len(parts) > 1 {
		rest = parts[1]
	}

	if !matchesFilter(rest, filter) {
		return Rule{}, false
	}

	return Rule{
		Kind:    kind,
		Value:   value,
		Comment: rest,
	}, true
}

func matchesFilter(rest string, filter string) bool {
	if filter == "" {
		return true
	}

	trimmedRest := strings.TrimSpace(rest)
	filterString := "@" + filter

	if !strings.HasPrefix(trimmedRest, "@") {
		return false
	}

	commentIndex := strings.Index(trimmedRest, "#")
	filterIndex := strings.Index(trimmedRest, filterString)

	if filterIndex == -1 || (commentIndex != -1 && commentIndex < filterIndex) {
		return false
	}

	return true
}

func (c *Converter) parseInclude(line string, filter string) ([]Item, error) {
	parts := strings.SplitN(line, " ", 2)
	subContentName := strings.TrimPrefix(parts[0], "include:")

	subContent, err := c.fileGetter(c.zipReader, subContentName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sub-upstream content: %w", err)
	}

	subConverter := NewConverter(c.zipReader, c.fileGetter)
	subItems, err := subConverter.Parse(subContent, filter)
	if err != nil {
		return nil, err
	}

	if !hasRules(subItems) {
		return nil, nil
	}

	items := make([]Item, 0, len(subItems)+1)
	items = append(items, Item{
		Kind:    ItemComment,
		Comment: "# " + line,
	})
	items = append(items, subItems...)
	return items, nil
}

func hasRules(items []Item) bool {
	for _, item := range items {
		if item.Kind == ItemRule {
			return true
		}
	}
	return false
}

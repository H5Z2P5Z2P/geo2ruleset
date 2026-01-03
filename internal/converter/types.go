// Package converter handles the conversion of v2fly domain list format to ruleset formats.
package converter

// ItemKind represents the kind of parsed item.
type ItemKind int

const (
	ItemRule ItemKind = iota
	ItemComment
)

// RuleKind represents the type of a rule.
type RuleKind int

const (
	RuleDomainSuffix RuleKind = iota
	RuleDomain
	RuleDomainKeyword
	RuleDomainRegex
)

// Rule represents a parsed rule line with optional comment.
type Rule struct {
	Kind    RuleKind
	Value   string
	Comment string
}

// Item is a parsed unit from upstream content.
type Item struct {
	Kind    ItemKind
	Rule    *Rule
	Comment string
}

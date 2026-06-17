package rules

import (
	"fmt"
	"strings"
)

type RuleType string

const (
	TypeDomain        RuleType = "DOMAIN"
	TypeDomainSuffix  RuleType = "DOMAIN-SUFFIX"
	TypeDomainKeyword RuleType = "DOMAIN-KEYWORD"
	TypeDomainRegex   RuleType = "DOMAIN-REGEX"
	TypeIPCIDR        RuleType = "IP-CIDR"
	TypeIPCIDR6       RuleType = "IP-CIDR6"
	TypeSrcIPCidr     RuleType = "SRC-IP-CIDR"
	TypeSrcPort       RuleType = "SRC-PORT"
	TypeGEOIP         RuleType = "GEOIP"
	TypeGEOSITE       RuleType = "GEOSITE"
	TypeRuleSet       RuleType = "RULE-SET"
	TypeMatch         RuleType = "MATCH"
	TypeMatchSrc      RuleType = "MATCH-SRC"
)

func (rt RuleType) IsValid() bool {
	switch rt {
	case TypeDomain, TypeDomainSuffix, TypeDomainKeyword, TypeDomainRegex,
		TypeIPCIDR, TypeIPCIDR6, TypeSrcIPCidr, TypeSrcPort,
		TypeGEOIP, TypeGEOSITE, TypeRuleSet, TypeMatch, TypeMatchSrc:
		return true
	}
	return false
}

type Rule struct {
	Type      RuleType
	Payload   string
	Proxy     string
	NoResolve bool
	Enabled   bool
}

func (r Rule) String() string {
	parts := []string{string(r.Type), r.Payload, r.Proxy}
	if r.NoResolve {
		parts = append(parts, "no-resolve")
	}
	return strings.Join(parts, ",")
}

type Provider struct {
	Name     string
	Type     string
	Behavior string
	URL      string
	Path     string
	Interval int
}

type Diff struct {
	Added    []Rule
	Removed  []Rule
	Modified []Rule
	Backup   string
}

type ImportSource struct {
	Kind string
	Ref  string
}

func (s ImportSource) String() string {
	return fmt.Sprintf("%s:%s", s.Kind, s.Ref)
}

package tui

import "testing"

func TestRulesI18nKeys_NotEmpty(t *testing.T) {
	for _, lang := range []Language{LangEn, LangZh} {
		SetLanguage(lang)
		m := T()
		keys := []string{
			m.RulesTitle, m.RulesMenuList, m.RulesMenuProviders,
			m.RulesMenuImport, m.RulesMenuReset, m.RulesHelpOpen,
		}
		for i, k := range keys {
			if k == "" {
				t.Errorf("lang=%v key[%d] empty", lang, i)
			}
		}
	}
}

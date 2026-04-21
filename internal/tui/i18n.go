package tui

// Language represents a supported UI language.
type Language string

const (
	LangEn Language = "en"
	LangZh Language = "zh"
)

// Dict stores all user-facing strings for a single language.
type Dict struct {
	// Header
	AppTitle          string
	PressSForSettings string

	// Status pills
	PillEndpoint string
	PillMode     string
	PillProxy    string
	PillLan      string
	PillTun      string
	PillFocus    string

	// Groups panel
	PanelGroups         string
	PanelGroupsHint     string
	NoGroupsMatchFilter string

	// Options panel
	PanelOptions      string
	SelectGroupFirst  string
	OptionsTitleFmt   string
	CurrentFmt        string
	NoSelectableNodes string

	// Settings overlay
	SettingsTitle       string
	SettingsHint        string
	SettingLabelMode    string
	SettingLabelSysProxy string
	SettingLabelAllowLan string
	SettingLabelTun     string
	SettingLabelRestart string
	HintCycle           string
	HintNewShells       string
	HintRestart         string
	NoSettingsAvailable string
	ValueRestart        string

	// Connections panel
	PanelConnections    string
	ConnectionStatsFmt  string
	NoActiveConnections string

	// Footer / status line
	StatusReady   string
	SearchLabel   string

	// Status messages
	StatusConnecting  string
	StatusConnected   string
	SearchCancelled   string
	FilterNone        string
	SettingsClosed    string
	TypeToFilter      string
	SettingsOpenHint  string
	FocusOptions      string
	FocusGroups       string
	FilterLabelFmt    string

	// Operation results
	SwitchedFmt       string
	ModeToFmt         string
	SysProxyPrefFmt   string
	AllowLanPrefFmt   string
	TunPrefFmt        string
	RestartUnavailable string
	RuntimeRestarted  string
	HintRestartShell  string
	HintRestartMixin  string

	// KeyMap help texts
	HelpMoveUp          string
	HelpMoveDown        string
	HelpFocusLeft       string
	HelpFocusRight      string
	HelpSwitchPane      string
	HelpApplySelect     string
	HelpRefreshDelay    string
	HelpSearch          string
	HelpSettings        string
	HelpCycleMode       string
	HelpToggleProxyPref string
	HelpCloseBack       string
	HelpQuit            string

	// Misc
	Loading           string
	SearchPlaceholder string
	BoolOn            string
	BoolOff           string
}

var currentLang = LangEn

var dicts = map[Language]Dict{
	LangEn: {
		AppTitle:          "LabProxy",
		PressSForSettings: "press s for settings",

		PillEndpoint: "endpoint",
		PillMode:     "mode",
		PillProxy:    "proxy",
		PillLan:      "lan",
		PillTun:      "tun",
		PillFocus:    "focus",

		PanelGroups:         "Groups",
		PanelGroupsHint:     "Tab / \u2190\u2192 to switch focus",
		NoGroupsMatchFilter: "No groups match the current filter.",

		PanelOptions:      "Options",
		SelectGroupFirst:  "Select a group first",
		OptionsTitleFmt:   "Options \u00b7 %s",
		CurrentFmt:        "current %s",
		NoSelectableNodes: "No selectable nodes in this group.",

		SettingsTitle:        "\u2699 Settings",
		SettingsHint:         "\u2191\u2193 move \u00b7 enter apply \u00b7 esc close",
		SettingLabelMode:     "Mode",
		SettingLabelSysProxy: "System Proxy",
		SettingLabelAllowLan: "Allow LAN",
		SettingLabelTun:      "Tun",
		SettingLabelRestart:  "Apply / Restart",
		HintCycle:            "cycle",
		HintNewShells:        "new shells",
		HintRestart:          "restart",
		NoSettingsAvailable:  "No settings available.",
		ValueRestart:         "\u21bb restart",

		PanelConnections:    "Connections",
		ConnectionStatsFmt:  "%d active \u00b7 \u2193 %s \u00b7 \u2191 %s",
		NoActiveConnections: "No active connections.",

		StatusReady: "ready",
		SearchLabel: "Search:",

		StatusConnecting: "connecting\u2026",
		StatusConnected:  "connected",
		SearchCancelled:  "search cancelled",
		FilterNone:       "none",
		SettingsClosed:   "settings closed",
		TypeToFilter:     "type to filter groups or proxies",
		SettingsOpenHint: "settings \u2014 enter apply \u00b7 esc close",
		FocusOptions:     "focus: options",
		FocusGroups:      "focus: groups",
		FilterLabelFmt:   "filter: %s",

		SwitchedFmt:        "switched %s \u2192 %s",
		ModeToFmt:          "mode \u2192 %s",
		SysProxyPrefFmt:    "system proxy pref \u2192 %s (applies to new shells / next start)",
		AllowLanPrefFmt:    "allow-lan pref \u2192 %s (saved, restart to apply)",
		TunPrefFmt:         "tun pref \u2192 %s (saved, restart to apply)",
		RestartUnavailable: "restart command unavailable; run labproxy restart in shell",
		RuntimeRestarted:   "runtime restarted and settings reapplied",
		HintRestartShell:   "run labproxy restart in shell",
		HintRestartMixin:   "apply saved mixin changes",

		HelpMoveUp:          "move up",
		HelpMoveDown:        "move down",
		HelpFocusLeft:       "focus left",
		HelpFocusRight:      "focus right",
		HelpSwitchPane:      "switch pane",
		HelpApplySelect:     "apply/select",
		HelpRefreshDelay:    "refresh delay",
		HelpSearch:          "search",
		HelpSettings:        "settings",
		HelpCycleMode:       "cycle mode",
		HelpToggleProxyPref: "toggle proxy pref",
		HelpCloseBack:       "close / back",
		HelpQuit:            "quit",

		Loading:           "loading\u2026",
		SearchPlaceholder: "Search proxies or groups",
		BoolOn:            "on",
		BoolOff:           "off",
	},
	LangZh: {
		AppTitle:          "LabProxy",
		PressSForSettings: "\u6309 s \u6253\u5f00\u8bbe\u7f6e",

		PillEndpoint: "\u7aef\u70b9",
		PillMode:     "\u6a21\u5f0f",
		PillProxy:    "\u4ee3\u7406",
		PillLan:      "\u5c40\u57df\u7f51",
		PillTun:      "TUN",
		PillFocus:    "\u7126\u70b9",

		PanelGroups:         "\u4ee3\u7406\u7ec4",
		PanelGroupsHint:     "Tab / \u2190\u2192 \u5207\u6362\u7126\u70b9",
		NoGroupsMatchFilter: "\u6ca1\u6709\u5339\u914d\u7684\u4ee3\u7406\u7ec4",

		PanelOptions:      "\u5019\u9009\u8282\u70b9",
		SelectGroupFirst:  "\u8bf7\u5148\u9009\u62e9\u4e00\u4e2a\u7ec4",
		OptionsTitleFmt:   "\u5019\u9009\u8282\u70b9 \u00b7 %s",
		CurrentFmt:        "\u5f53\u524d: %s",
		NoSelectableNodes: "\u8be5\u7ec4\u65e0\u53ef\u9009\u8282\u70b9",

		SettingsTitle:        "\u2699 \u8bbe\u7f6e",
		SettingsHint:         "\u2191\u2193 \u79fb\u52a8 \u00b7 \u56de\u8f66\u786e\u8ba4 \u00b7 esc \u5173\u95ed",
		SettingLabelMode:     "\u6a21\u5f0f",
		SettingLabelSysProxy: "\u7cfb\u7edf\u4ee3\u7406",
		SettingLabelAllowLan: "\u5141\u8bb8\u5c40\u57df\u7f51",
		SettingLabelTun:      "TUN \u6a21\u5f0f",
		SettingLabelRestart:  "\u5e94\u7528 / \u91cd\u542f",
		HintCycle:            "\u5faa\u73af\u5207\u6362",
		HintNewShells:        "\u65b0\u7ec8\u7aef\u751f\u6548",
		HintRestart:          "\u91cd\u542f\u540e\u751f\u6548",
		NoSettingsAvailable:  "\u65e0\u53ef\u7528\u8bbe\u7f6e",
		ValueRestart:         "\u21bb \u91cd\u542f",

		PanelConnections:    "\u6d3b\u8dc3\u8fde\u63a5",
		ConnectionStatsFmt:  "%d \u4e2a\u6d3b\u8dc3 \u00b7 \u2193 %s \u00b7 \u2191 %s",
		NoActiveConnections: "\u65e0\u6d3b\u8dc3\u8fde\u63a5",

		StatusReady: "\u5c31\u7eea",
		SearchLabel: "\u641c\u7d22:",

		StatusConnecting: "\u8fde\u63a5\u4e2d\u2026",
		StatusConnected:  "\u5df2\u8fde\u63a5",
		SearchCancelled:  "\u641c\u7d22\u5df2\u53d6\u6d88",
		FilterNone:       "\u65e0",
		SettingsClosed:   "\u8bbe\u7f6e\u5df2\u5173\u95ed",
		TypeToFilter:     "\u8f93\u5165\u4ee5\u8fc7\u6ee4\u4ee3\u7406\u7ec4\u6216\u8282\u70b9",
		SettingsOpenHint: "\u8bbe\u7f6e \u2014 \u56de\u8f66\u786e\u8ba4 \u00b7 esc \u5173\u95ed",
		FocusOptions:     "\u7126\u70b9: \u5019\u9009\u8282\u70b9",
		FocusGroups:      "\u7126\u70b9: \u4ee3\u7406\u7ec4",
		FilterLabelFmt:   "\u8fc7\u6ee4: %s",

		SwitchedFmt:        "\u5df2\u5207\u6362 %s \u2192 %s",
		ModeToFmt:          "\u6a21\u5f0f \u2192 %s",
		SysProxyPrefFmt:    "\u7cfb\u7edf\u4ee3\u7406\u504f\u597d \u2192 %s\uff08\u5bf9\u65b0\u7ec8\u7aef/\u4e0b\u6b21\u542f\u52a8\u751f\u6548\uff09",
		AllowLanPrefFmt:    "\u5c40\u57df\u7f51\u504f\u597d \u2192 %s\uff08\u5df2\u4fdd\u5b58\uff0c\u91cd\u542f\u540e\u751f\u6548\uff09",
		TunPrefFmt:         "TUN \u504f\u597d \u2192 %s\uff08\u5df2\u4fdd\u5b58\uff0c\u91cd\u542f\u540e\u751f\u6548\uff09",
		RestartUnavailable: "\u91cd\u542f\u547d\u4ee4\u4e0d\u53ef\u7528\uff1b\u8bf7\u5728 shell \u4e2d\u8fd0\u884c labproxy restart",
		RuntimeRestarted:   "\u8fd0\u884c\u65f6\u5df2\u91cd\u542f\u4e14\u8bbe\u7f6e\u5df2\u91cd\u65b0\u5e94\u7528",
		HintRestartShell:   "\u5728 shell \u4e2d\u8fd0\u884c labproxy restart",
		HintRestartMixin:   "\u5e94\u7528\u5df2\u4fdd\u5b58\u7684 mixin \u53d8\u66f4",

		HelpMoveUp:          "\u4e0a\u79fb",
		HelpMoveDown:        "\u4e0b\u79fb",
		HelpFocusLeft:       "\u7126\u70b9\u5de6\u79fb",
		HelpFocusRight:      "\u7126\u70b9\u53f3\u79fb",
		HelpSwitchPane:      "\u5207\u6362\u9762\u677f",
		HelpApplySelect:     "\u786e\u8ba4/\u9009\u4e2d",
		HelpRefreshDelay:    "\u5ef6\u8fdf\u6d4b\u8bd5",
		HelpSearch:          "\u641c\u7d22",
		HelpSettings:        "\u8bbe\u7f6e",
		HelpCycleMode:       "\u5207\u6362\u6a21\u5f0f",
		HelpToggleProxyPref: "\u5207\u6362\u7cfb\u7edf\u4ee3\u7406",
		HelpCloseBack:       "\u5173\u95ed/\u8fd4\u56de",
		HelpQuit:            "\u9000\u51fa",

		Loading:           "\u52a0\u8f7d\u4e2d\u2026",
		SearchPlaceholder: "\u641c\u7d22\u4ee3\u7406\u7ec4\u6216\u8282\u70b9",
		BoolOn:            "\u5f00",
		BoolOff:           "\u5173",
	},
}

// SetLanguage sets the active UI language.
func SetLanguage(lang Language) {
	if _, ok := dicts[lang]; ok {
		currentLang = lang
	}
}

// T returns the dictionary for the current language.
func T() *Dict {
	d := dicts[currentLang]
	return &d
}

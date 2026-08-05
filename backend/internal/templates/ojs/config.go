// Package ojs provides OJS-specific template implementation.
package ojs

import "ojs-monitor/backend/internal/domain/template"

// getDefaultConfig returns the default configuration for OJS projects.
func getDefaultConfig() *template.TemplateConfig {
	return &template.TemplateConfig{
		Template: "ojs",
		DisplayName: "Open Journal Systems (OJS) 3.x",

		DefaultWatchPaths: []string{
			"public/",
			"lib/pkp/",
			"plugins/",
		},

		DefaultFilesPaths: []string{
			"files/",
		},

		DefaultBlacklistExts: []string{
			"php", "phtml", "php3", "php4", "php5", "php7", "pht", "phar",
			"sh", "bash", "zsh",
			"pl", "py", "rb",
			"exe", "bat", "cmd", "ps1",
		},

		DefaultWhitelistPaths: []string{
			"lib/pkp/classes/",  // OJS core classes
			"plugins/generic/",   // Trusted plugins
			"plugins/themes/",    // Trusted themes
		},

		DefaultRescanInterval: 10, // minutes

		WatchType: "OJS_WORKFLOW",
	}
}

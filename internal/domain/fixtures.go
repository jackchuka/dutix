package domain

// Test fixture apps
var (
	VSCodeApp = &App{
		Name:     "Visual Studio Code",
		BundleID: "com.microsoft.VSCode",
		Path:     "/Applications/Visual Studio Code.app",
	}

	TextEditApp = &App{
		Name:     "TextEdit",
		BundleID: "com.apple.TextEdit",
		Path:     "/Applications/TextEdit.app",
	}

	SafariApp = &App{
		Name:     "Safari",
		BundleID: "com.apple.Safari",
		Path:     "/Applications/Safari.app",
	}

	ChromeApp = &App{
		Name:     "Google Chrome",
		BundleID: "com.google.Chrome",
		Path:     "/Applications/Google Chrome.app",
	}
)

// Test fixture targets
var (
	TxtExtensionTarget = Target{
		Kind:       TargetKindExtension,
		Identifier: "txt",
	}

	PlainTextUTITarget = Target{
		Kind:       TargetKindUTI,
		Identifier: "public.plain-text",
	}

	HTTPSchemeTarget = Target{
		Kind:       TargetKindScheme,
		Identifier: "http",
	}

	HTTPSSchemeTarget = Target{
		Kind:       TargetKindScheme,
		Identifier: "https",
	}
)

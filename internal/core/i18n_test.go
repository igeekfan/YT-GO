package core

import "testing"

func TestLangFromLocale(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		want   Lang
	}{
		{name: "simplified Chinese", locale: "zh_CN.UTF-8", want: LangZhCN},
		{name: "Chinese language tag", locale: "zh-Hans", want: LangZhCN},
		{name: "Windows Chinese locale", locale: "Chinese (Simplified)_China.936", want: LangZhCN},
		{name: "English locale", locale: "en_US.UTF-8", want: LangEnUS},
		{name: "C locale", locale: "C.UTF-8", want: LangEnUS},
		{name: "locale preference list", locale: "zh_CN:en_US", want: LangZhCN},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := langFromLocale(test.locale); got != test.want {
				t.Fatalf("langFromLocale(%q) = %q, want %q", test.locale, got, test.want)
			}
		})
	}
}

func TestLangFromEnvironmentUsesLocalePrecedence(t *testing.T) {
	env := map[string]string{
		"LC_ALL":      "",
		"LC_MESSAGES": "",
		"LANG":        "zh_CN.UTF-8",
		"LANGUAGE":    "",
	}
	getenv := func(key string) string { return env[key] }

	if got := langFromEnvironment(getenv); got != LangZhCN {
		t.Fatalf("langFromEnvironment() = %q, want %q", got, LangZhCN)
	}

	env["LC_MESSAGES"] = "en_US.UTF-8"
	if got := langFromEnvironment(getenv); got != LangEnUS {
		t.Fatalf("LC_MESSAGES should override LANG, got %q", got)
	}

	env["LC_ALL"] = "zh_TW.UTF-8"
	if got := langFromEnvironment(getenv); got != LangZhCN {
		t.Fatalf("LC_ALL should override LC_MESSAGES, got %q", got)
	}
}

func TestLangFromEnvironmentDefaultsToEnglish(t *testing.T) {
	if got := langFromEnvironment(func(string) string { return "" }); got != LangEnUS {
		t.Fatalf("empty locale environment = %q, want %q", got, LangEnUS)
	}
}

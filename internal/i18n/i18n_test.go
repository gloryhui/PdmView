package i18n

import (
	"slices"
	"sort"
	"testing"
)

func TestResourceKeyAndParameterParity(t *testing.T) {
	if len(resources[EN]) != len(resources[ZHCN]) {
		t.Fatalf("resource counts differ: en=%d zh-CN=%d", len(resources[EN]), len(resources[ZHCN]))
	}
	for key, en := range resources[EN] {
		zh, ok := resources[ZHCN][key]
		if !ok {
			t.Errorf("zh-CN is missing key %q", key)
			continue
		}
		enParams := slices.Clone(en.params)
		zhParams := slices.Clone(zh.params)
		sort.Strings(enParams)
		sort.Strings(zhParams)
		if !slices.Equal(enParams, zhParams) {
			t.Errorf("%q parameters differ: en=%v zh-CN=%v", key, enParams, zhParams)
		}
	}
}

func TestEnglishFallbackAndNamedParameters(t *testing.T) {
	saved, hadSaved := resources[ZHCN][KeyStatusSaved]
	delete(resources[ZHCN], KeyStatusSaved)
	t.Cleanup(func() {
		if hadSaved {
			resources[ZHCN][KeyStatusSaved] = saved
		}
	})

	got := Translate(ZHCN, KeyStatusSaved, map[string]string{"language": "简体中文"})
	want := "Interface language: 简体中文. Language preference saved."
	if got != want {
		t.Fatalf("fallback = %q, want %q", got, want)
	}
}

func TestResolvePreferenceAndSystemPriority(t *testing.T) {
	tests := []struct {
		name      string
		saved     string
		systemTag string
		want      Locale
	}{
		{name: "saved wins", saved: "zh-CN", systemTag: "en-US", want: ZHCN},
		{name: "invalid saved falls back", saved: "fr", systemTag: "zh-Hans-CN", want: ZHCN},
		{name: "chinese primary", systemTag: "zh-TW", want: ZHCN},
		{name: "non-chinese primary", systemTag: "ja-JP", want: EN},
		{name: "missing system", systemTag: "", want: EN},
		{name: "underscore tag", systemTag: "zh_CN", want: ZHCN},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Resolve(tc.saved, tc.systemTag); got != tc.want {
				t.Fatalf("Resolve(%q, %q) = %q, want %q", tc.saved, tc.systemTag, got, tc.want)
			}
		})
	}
}

func TestLanguageNamesAreFixedOptions(t *testing.T) {
	if got := LanguageName(ZHCN); got != "简体中文" {
		t.Fatalf("zh-CN name = %q", got)
	}
	if got := LanguageName(EN); got != "English" {
		t.Fatalf("en name = %q", got)
	}
}

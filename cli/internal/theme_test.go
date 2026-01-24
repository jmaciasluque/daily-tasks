package internal

import "testing"

func TestGetTheme(t *testing.T) {
	t.Run("valid index", func(t *testing.T) {
		theme := GetTheme(0)
		if theme.Name != "Charcoal" {
			t.Errorf("expected Charcoal, got %s", theme.Name)
		}

		theme = GetTheme(1)
		if theme.Name != "Sand" {
			t.Errorf("expected Sand, got %s", theme.Name)
		}
	})

	t.Run("negative index returns first theme", func(t *testing.T) {
		theme := GetTheme(-1)
		if theme.Name != "Charcoal" {
			t.Errorf("expected Charcoal for negative index, got %s", theme.Name)
		}
	})

	t.Run("out of bounds returns first theme", func(t *testing.T) {
		theme := GetTheme(9999)
		if theme.Name != "Charcoal" {
			t.Errorf("expected Charcoal for out of bounds, got %s", theme.Name)
		}
	})
}

func TestThemeCount(t *testing.T) {
	count := ThemeCount()
	if count != 25 {
		t.Errorf("expected 25 themes, got %d", count)
	}
}

func TestThemeFieldsNotEmpty(t *testing.T) {
	for i, theme := range Themes {
		if theme.Name == "" {
			t.Errorf("theme %d has empty Name", i)
		}
		if theme.Bg == "" {
			t.Errorf("theme %d has empty Bg", i)
		}
		if theme.PanelBg == "" {
			t.Errorf("theme %d has empty PanelBg", i)
		}
		if theme.Text == "" {
			t.Errorf("theme %d has empty Text", i)
		}
		if theme.Muted == "" {
			t.Errorf("theme %d has empty Muted", i)
		}
		if theme.Border == "" {
			t.Errorf("theme %d has empty Border", i)
		}
		if theme.FocusBorder == "" {
			t.Errorf("theme %d has empty FocusBorder", i)
		}
		if theme.FocusBg == "" {
			t.Errorf("theme %d has empty FocusBg", i)
		}
		if theme.Accent == "" {
			t.Errorf("theme %d has empty Accent", i)
		}
	}
}

func TestThemeColorsAreValidHex(t *testing.T) {
	isValidHex := func(s string) bool {
		if len(s) != 7 || s[0] != '#' {
			return false
		}
		for _, c := range s[1:] {
			if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')) {
				return false
			}
		}
		return true
	}

	for i, theme := range Themes {
		colors := []struct {
			name  string
			value string
		}{
			{"Bg", theme.Bg},
			{"PanelBg", theme.PanelBg},
			{"Text", theme.Text},
			{"Muted", theme.Muted},
			{"Border", theme.Border},
			{"FocusBorder", theme.FocusBorder},
			{"FocusBg", theme.FocusBg},
			{"Accent", theme.Accent},
		}

		for _, c := range colors {
			if !isValidHex(c.value) {
				t.Errorf("theme %d (%s) has invalid hex color for %s: %s", i, theme.Name, c.name, c.value)
			}
		}
	}
}

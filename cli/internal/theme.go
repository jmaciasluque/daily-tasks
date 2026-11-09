package internal

// Theme represents a color theme for the UI
type Theme struct {
	Name        string
	Bg          string
	PanelBg     string
	Text        string
	Muted       string
	Border      string
	FocusBorder string
	FocusBg     string
	Accent      string
}

// Themes is the list of available themes
var Themes = []Theme{
	{Name: "Charcoal", Bg: "#111111", PanelBg: "#1A1A1A", Text: "#E5E7EB", Muted: "#9CA3AF", Border: "#2A2A2A", FocusBorder: "#4B5563", FocusBg: "#1F2937", Accent: "#F59E0B"},
	{Name: "Sand", Bg: "#F6F1E7", PanelBg: "#FFF8EE", Text: "#3B2F2F", Muted: "#8C7B6B", Border: "#D6C7B2", FocusBorder: "#B08968", FocusBg: "#F1E2C8", Accent: "#B45309"},
	{Name: "Mint", Bg: "#0F1E1B", PanelBg: "#14312C", Text: "#D1FAE5", Muted: "#7BB9A5", Border: "#1F3F37", FocusBorder: "#34D399", FocusBg: "#1B4B3F", Accent: "#10B981"},
	{Name: "Ocean", Bg: "#0B1C2C", PanelBg: "#11243A", Text: "#DDEBFF", Muted: "#7AA3C4", Border: "#1E3650", FocusBorder: "#3B82F6", FocusBg: "#162E4A", Accent: "#38BDF8"},
	{Name: "Ember", Bg: "#1B0E0E", PanelBg: "#2A1414", Text: "#FEE2E2", Muted: "#FCA5A5", Border: "#3B1C1C", FocusBorder: "#F87171", FocusBg: "#3A1C1C", Accent: "#FB923C"},
	{Name: "Mono Light", Bg: "#F4F4F5", PanelBg: "#FFFFFF", Text: "#111827", Muted: "#6B7280", Border: "#D1D5DB", FocusBorder: "#111827", FocusBg: "#E5E7EB", Accent: "#0EA5E9"},
	{Name: "Solarized Dark", Bg: "#002B36", PanelBg: "#073642", Text: "#EEE8D5", Muted: "#93A1A1", Border: "#0B3B45", FocusBorder: "#268BD2", FocusBg: "#0B3B45", Accent: "#2AA198"},
	{Name: "Solarized Light", Bg: "#FDF6E3", PanelBg: "#FFF8DC", Text: "#586E75", Muted: "#93A1A1", Border: "#E7DEC3", FocusBorder: "#268BD2", FocusBg: "#EEE8D5", Accent: "#B58900"},
	{Name: "Forest", Bg: "#0F1A12", PanelBg: "#16261B", Text: "#E2F5E7", Muted: "#88A08E", Border: "#1C2F22", FocusBorder: "#22C55E", FocusBg: "#1E3A2A", Accent: "#84CC16"},
	{Name: "Plum", Bg: "#1A0F1F", PanelBg: "#2A1630", Text: "#F3E8FF", Muted: "#C4B5FD", Border: "#3B1F44", FocusBorder: "#A78BFA", FocusBg: "#3A2046", Accent: "#F472B6"},
	{Name: "Slate", Bg: "#0F172A", PanelBg: "#111827", Text: "#E5E7EB", Muted: "#9CA3AF", Border: "#1F2937", FocusBorder: "#94A3B8", FocusBg: "#1F2937", Accent: "#38BDF8"},
	{Name: "Coral", Bg: "#2A1410", PanelBg: "#3B1D17", Text: "#FFE4E6", Muted: "#FCA5A5", Border: "#4A2420", FocusBorder: "#FB7185", FocusBg: "#4A241E", Accent: "#FDBA74"},
	{Name: "Meadow", Bg: "#F1FAF3", PanelBg: "#FFFFFF", Text: "#1F2937", Muted: "#6B7280", Border: "#CDE7D4", FocusBorder: "#22C55E", FocusBg: "#E8F7EC", Accent: "#16A34A"},
	{Name: "Cobalt", Bg: "#0A0F2D", PanelBg: "#0F173B", Text: "#DDE2FF", Muted: "#8AA2FF", Border: "#1B255C", FocusBorder: "#6366F1", FocusBg: "#1C2452", Accent: "#60A5FA"},
	{Name: "Amber", Bg: "#1F1600", PanelBg: "#2A1E00", Text: "#FEF3C7", Muted: "#FCD34D", Border: "#3A2A00", FocusBorder: "#F59E0B", FocusBg: "#3A2A00", Accent: "#FBBF24"},
	{Name: "Paper", Bg: "#FAF7F0", PanelBg: "#FFFFFF", Text: "#2F2A24", Muted: "#8B8175", Border: "#E5DED5", FocusBorder: "#9A6F3A", FocusBg: "#EFE4D6", Accent: "#C2410C"},
	{Name: "Ice", Bg: "#0B1418", PanelBg: "#112027", Text: "#E6F4F1", Muted: "#8FB7B0", Border: "#1C2F36", FocusBorder: "#5EEAD4", FocusBg: "#1B2D33", Accent: "#2DD4BF"},
	{Name: "Lavender", Bg: "#201626", PanelBg: "#2B1F33", Text: "#F5E9FF", Muted: "#C4B5FD", Border: "#3A2B45", FocusBorder: "#C084FC", FocusBg: "#3B2A49", Accent: "#A855F7"},
	{Name: "Rose", Bg: "#2A0E1C", PanelBg: "#3A1327", Text: "#FFE4E6", Muted: "#FDA4AF", Border: "#4A1A32", FocusBorder: "#FB7185", FocusBg: "#4A1A32", Accent: "#F43F5E"},
	{Name: "Citrus", Bg: "#0F1405", PanelBg: "#1A220A", Text: "#ECFCCB", Muted: "#BEF264", Border: "#26300F", FocusBorder: "#A3E635", FocusBg: "#2A3412", Accent: "#FACC15"},
	{Name: "Steel", Bg: "#111214", PanelBg: "#1A1C1F", Text: "#E5E7EB", Muted: "#9CA3AF", Border: "#2A2D32", FocusBorder: "#7C8AA6", FocusBg: "#22252B", Accent: "#60A5FA"},
	{Name: "Redwood", Bg: "#20110E", PanelBg: "#2B1612", Text: "#FFE4E1", Muted: "#D6A2A0", Border: "#3A1C16", FocusBorder: "#C97C5D", FocusBg: "#3B1E18", Accent: "#F97316"},
	{Name: "Lagoon", Bg: "#061A1A", PanelBg: "#0B2626", Text: "#D1FAE5", Muted: "#7BC4B8", Border: "#123737", FocusBorder: "#2DD4BF", FocusBg: "#123737", Accent: "#14B8A6"},
	{Name: "Sunrise", Bg: "#2A1506", PanelBg: "#3B1E09", Text: "#FFE8D6", Muted: "#FDBA74", Border: "#4A2710", FocusBorder: "#FB923C", FocusBg: "#4A2710", Accent: "#F97316"},
	{Name: "Graphite", Bg: "#0B0B0C", PanelBg: "#141416", Text: "#F3F4F6", Muted: "#A1A1AA", Border: "#27272A", FocusBorder: "#52525B", FocusBg: "#1F1F23", Accent: "#22D3EE"},
}

// GetTheme returns the theme at the given index, or the first theme if out of bounds
func GetTheme(index int) Theme {
	if index < 0 || index >= len(Themes) {
		return Themes[0]
	}
	return Themes[index]
}

// ThemeCount returns the number of available themes
func ThemeCount() int {
	return len(Themes)
}

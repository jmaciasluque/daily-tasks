import { THEMES, getTheme, isLightColor } from '../theme/themes';

describe('THEMES', () => {
  it('has 25 themes', () => {
    expect(THEMES.length).toBe(25);
  });

  it('all themes have required fields', () => {
    THEMES.forEach((theme, index) => {
      expect(theme.name).toBeTruthy();
      expect(theme.bg).toBeTruthy();
      expect(theme.panelBg).toBeTruthy();
      expect(theme.text).toBeTruthy();
      expect(theme.muted).toBeTruthy();
      expect(theme.border).toBeTruthy();
      expect(theme.focusBorder).toBeTruthy();
      expect(theme.focusBg).toBeTruthy();
      expect(theme.accent).toBeTruthy();
    });
  });

  it('all colors are valid hex', () => {
    const isValidHex = (s: string) => /^#[0-9A-Fa-f]{6}$/.test(s);

    THEMES.forEach((theme, index) => {
      const colors = [
        theme.bg, theme.panelBg, theme.text, theme.muted,
        theme.border, theme.focusBorder, theme.focusBg, theme.accent
      ];
      colors.forEach(color => {
        expect(isValidHex(color)).toBe(true);
      });
    });
  });
});

describe('getTheme', () => {
  it('returns theme at valid index', () => {
    expect(getTheme(0).name).toBe('Charcoal');
    expect(getTheme(1).name).toBe('Sand');
  });

  it('returns first theme for negative index', () => {
    expect(getTheme(-1).name).toBe('Charcoal');
  });

  it('returns first theme for out of bounds index', () => {
    expect(getTheme(9999).name).toBe('Charcoal');
  });
});

describe('isLightColor', () => {
  it('returns true for light colors', () => {
    expect(isLightColor('#FFFFFF')).toBe(true);
    expect(isLightColor('#F6F1E7')).toBe(true);
    expect(isLightColor('#F4F4F5')).toBe(true);
  });

  it('returns false for dark colors', () => {
    expect(isLightColor('#000000')).toBe(false);
    expect(isLightColor('#111111')).toBe(false);
    expect(isLightColor('#0F1E1B')).toBe(false);
  });

  it('handles invalid input', () => {
    expect(isLightColor('')).toBe(false);
    expect(isLightColor('invalid')).toBe(false);
    expect(isLightColor('#FFF')).toBe(false);
  });
});

export type Theme = {
  name: string;
  bg: string;
  panelBg: string;
  text: string;
  muted: string;
  border: string;
  focusBorder: string;
  focusBg: string;
  accent: string;
};

export const THEMES: Theme[] = [
  { name: 'Charcoal', bg: '#111111', panelBg: '#1A1A1A', text: '#E5E7EB', muted: '#9CA3AF', border: '#2A2A2A', focusBorder: '#4B5563', focusBg: '#1F2937', accent: '#F59E0B' },
  { name: 'Sand', bg: '#F6F1E7', panelBg: '#FFF8EE', text: '#3B2F2F', muted: '#8C7B6B', border: '#D6C7B2', focusBorder: '#B08968', focusBg: '#F1E2C8', accent: '#B45309' },
  { name: 'Mint', bg: '#0F1E1B', panelBg: '#14312C', text: '#D1FAE5', muted: '#7BB9A5', border: '#1F3F37', focusBorder: '#34D399', focusBg: '#1B4B3F', accent: '#10B981' },
  { name: 'Ocean', bg: '#0B1C2C', panelBg: '#11243A', text: '#DDEBFF', muted: '#7AA3C4', border: '#1E3650', focusBorder: '#3B82F6', focusBg: '#162E4A', accent: '#38BDF8' },
  { name: 'Ember', bg: '#1B0E0E', panelBg: '#2A1414', text: '#FEE2E2', muted: '#FCA5A5', border: '#3B1C1C', focusBorder: '#F87171', focusBg: '#3A1C1C', accent: '#FB923C' },
  { name: 'Mono Light', bg: '#F4F4F5', panelBg: '#FFFFFF', text: '#111827', muted: '#6B7280', border: '#D1D5DB', focusBorder: '#111827', focusBg: '#E5E7EB', accent: '#0EA5E9' },
  { name: 'Solarized Dark', bg: '#002B36', panelBg: '#073642', text: '#EEE8D5', muted: '#93A1A1', border: '#0B3B45', focusBorder: '#268BD2', focusBg: '#0B3B45', accent: '#2AA198' },
  { name: 'Solarized Light', bg: '#FDF6E3', panelBg: '#FFF8DC', text: '#586E75', muted: '#93A1A1', border: '#E7DEC3', focusBorder: '#268BD2', focusBg: '#EEE8D5', accent: '#B58900' },
  { name: 'Forest', bg: '#0F1A12', panelBg: '#16261B', text: '#E2F5E7', muted: '#88A08E', border: '#1C2F22', focusBorder: '#22C55E', focusBg: '#1E3A2A', accent: '#84CC16' },
  { name: 'Plum', bg: '#1A0F1F', panelBg: '#2A1630', text: '#F3E8FF', muted: '#C4B5FD', border: '#3B1F44', focusBorder: '#A78BFA', focusBg: '#3A2046', accent: '#F472B6' },
  { name: 'Slate', bg: '#0F172A', panelBg: '#111827', text: '#E5E7EB', muted: '#9CA3AF', border: '#1F2937', focusBorder: '#94A3B8', focusBg: '#1F2937', accent: '#38BDF8' },
  { name: 'Coral', bg: '#2A1410', panelBg: '#3B1D17', text: '#FFE4E6', muted: '#FCA5A5', border: '#4A2420', focusBorder: '#FB7185', focusBg: '#4A241E', accent: '#FDBA74' },
  { name: 'Meadow', bg: '#F1FAF3', panelBg: '#FFFFFF', text: '#1F2937', muted: '#6B7280', border: '#CDE7D4', focusBorder: '#22C55E', focusBg: '#E8F7EC', accent: '#16A34A' },
  { name: 'Cobalt', bg: '#0A0F2D', panelBg: '#0F173B', text: '#DDE2FF', muted: '#8AA2FF', border: '#1B255C', focusBorder: '#6366F1', focusBg: '#1C2452', accent: '#60A5FA' },
  { name: 'Amber', bg: '#1F1600', panelBg: '#2A1E00', text: '#FEF3C7', muted: '#FCD34D', border: '#3A2A00', focusBorder: '#F59E0B', focusBg: '#3A2A00', accent: '#FBBF24' },
  { name: 'Paper', bg: '#FAF7F0', panelBg: '#FFFFFF', text: '#2F2A24', muted: '#8B8175', border: '#E5DED5', focusBorder: '#9A6F3A', focusBg: '#EFE4D6', accent: '#C2410C' },
  { name: 'Ice', bg: '#0B1418', panelBg: '#112027', text: '#E6F4F1', muted: '#8FB7B0', border: '#1C2F36', focusBorder: '#5EEAD4', focusBg: '#1B2D33', accent: '#2DD4BF' },
  { name: 'Lavender', bg: '#201626', panelBg: '#2B1F33', text: '#F5E9FF', muted: '#C4B5FD', border: '#3A2B45', focusBorder: '#C084FC', focusBg: '#3B2A49', accent: '#A855F7' },
  { name: 'Rose', bg: '#2A0E1C', panelBg: '#3A1327', text: '#FFE4E6', muted: '#FDA4AF', border: '#4A1A32', focusBorder: '#FB7185', focusBg: '#4A1A32', accent: '#F43F5E' },
  { name: 'Citrus', bg: '#0F1405', panelBg: '#1A220A', text: '#ECFCCB', muted: '#BEF264', border: '#26300F', focusBorder: '#A3E635', focusBg: '#2A3412', accent: '#FACC15' },
  { name: 'Steel', bg: '#111214', panelBg: '#1A1C1F', text: '#E5E7EB', muted: '#9CA3AF', border: '#2A2D32', focusBorder: '#7C8AA6', focusBg: '#22252B', accent: '#60A5FA' },
  { name: 'Redwood', bg: '#20110E', panelBg: '#2B1612', text: '#FFE4E1', muted: '#D6A2A0', border: '#3A1C16', focusBorder: '#C97C5D', focusBg: '#3B1E18', accent: '#F97316' },
  { name: 'Lagoon', bg: '#061A1A', panelBg: '#0B2626', text: '#D1FAE5', muted: '#7BC4B8', border: '#123737', focusBorder: '#2DD4BF', focusBg: '#123737', accent: '#14B8A6' },
  { name: 'Sunrise', bg: '#2A1506', panelBg: '#3B1E09', text: '#FFE8D6', muted: '#FDBA74', border: '#4A2710', focusBorder: '#FB923C', focusBg: '#4A2710', accent: '#F97316' },
  { name: 'Graphite', bg: '#0B0B0C', panelBg: '#141416', text: '#F3F4F6', muted: '#A1A1AA', border: '#27272A', focusBorder: '#52525B', focusBg: '#1F1F23', accent: '#22D3EE' },
];

export function getTheme(index: number): Theme {
  if (index < 0 || index >= THEMES.length) {
    return THEMES[0];
  }
  return THEMES[index];
}

export function isLightColor(hex: string): boolean {
  const cleaned = hex.replace('#', '');
  if (cleaned.length !== 6) return false;
  const r = Number.parseInt(cleaned.slice(0, 2), 16);
  const g = Number.parseInt(cleaned.slice(2, 4), 16);
  const b = Number.parseInt(cleaned.slice(4, 6), 16);
  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
  return luminance > 0.7;
}

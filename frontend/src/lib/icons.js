/**
 * Visual identity: coalition colours, used to tint player and event markers.
 */

export const COALITION_COLORS = {
  blue: '#3d7dff',
  red: '#ff4d4d',
  neutral: '#9aa4b2',
};

export function coalitionColor(coalition) {
  return COALITION_COLORS[coalition] ?? COALITION_COLORS.neutral;
}

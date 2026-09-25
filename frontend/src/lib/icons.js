/**
 * Visual identity: coalition colours and category icons.
 *
 * Icons are inline SVG strings used with Leaflet `divIcon`, so no external
 * image assets are needed and the map works fully offline.
 */

export const COALITION_COLORS = {
  blue: '#3d7dff',
  red: '#ff4d4d',
  neutral: '#9aa4b2',
};

export function coalitionColor(coalition) {
  return COALITION_COLORS[coalition] ?? COALITION_COLORS.neutral;
}

const SHAPES = {
  // Nose-up triangle
  plane: '<path d="M12 2 L18 20 L12 16 L6 20 Z"/>',
  // Rotor cross with a tail
  heli: '<path d="M12 3 v18 M4 8 h16 M12 8 l6 12"/>',
  // Square (ground vehicle)
  ground: '<rect x="6" y="8" width="12" height="9" rx="1"/>',
  // Boat hull
  ship: '<path d="M4 14 h16 l-3 5 H7 Z M12 4 v10"/>',
  // Building
  structure: '<path d="M6 20 V9 l6-4 6 4 v11 Z"/>',
  // Fallback dot
  other: '<circle cx="12" cy="12" r="5"/>',
};

/**
 * Returns a Leaflet DivIcon for a unit.
 * @param {any} L the Leaflet namespace
 * @param {{category?: string, coalition?: string, ownship?: boolean}} unit
 */
export function unitIcon(L, unit) {
  const color = coalitionColor(unit.coalition);
  const shape = SHAPES[unit.category] ?? SHAPES.other;
  const stroke = unit.ownship ? '#ffffff' : 'rgba(0,0,0,.45)';
  const ownshipRing = unit.ownship
    ? `<circle cx="12" cy="12" r="11" fill="none" stroke="#ffffff" stroke-width="1.5" opacity=".9"/>`
    : '';

  const html = `
    <svg viewBox="0 0 24 24" width="22" height="22"
         fill="${color}" stroke="${stroke}" stroke-width="1.2"
         stroke-linejoin="round" stroke-linecap="round">
      ${ownshipRing}
      ${shape}
    </svg>`;

  return L.divIcon({
    html,
    className: 'dcsmm-marker',
    iconSize: [22, 22],
    iconAnchor: [11, 11],
  });
}

export const CATEGORY_LABELS = {
  plane: 'Avion',
  heli: 'Hélicoptère',
  ground: 'Véhicule',
  ship: 'Navire',
  structure: 'Structure',
  other: 'Objet',
};

export const COALITION_LABELS = {
  blue: 'Bleu',
  red: 'Rouge',
  neutral: 'Neutre',
};

/**
 * Internationalisation.
 *
 * Default language is English (the repository is public and most DCS players
 * are English-speaking); French remains one click away. The choice is persisted
 * in localStorage.
 *
 * Usage in a component:
 *   import { t } from './lib/i18n.js';
 *   {$t('map.recenter')}
 */
import { derived, get, writable } from 'svelte/store';

export const LANGUAGES = [
  { id: 'en', label: 'English' },
  { id: 'fr', label: 'Français' },
];

const DEFAULT_LANG = 'en';
const STORAGE_KEY = 'dcsmm.lang';

function initialLang() {
  if (typeof localStorage === 'undefined') return DEFAULT_LANG;
  const saved = localStorage.getItem(STORAGE_KEY);
  if (saved && LANGUAGES.some((l) => l.id === saved)) return saved;
  return DEFAULT_LANG;
}

export const lang = writable(initialLang());

lang.subscribe((value) => {
  if (typeof localStorage !== 'undefined') localStorage.setItem(STORAGE_KEY, value);
  if (typeof document !== 'undefined') document.documentElement.lang = value;
});

// ---------------------------------------------------------------------------
// Dictionaries. Keys are grouped by area; every key must exist in both files.
// ---------------------------------------------------------------------------

const en = {
  'app.title': 'DCS Mission Manager',
  'app.connected': 'connected',
  'app.offline': 'offline',
  'app.basemap': 'Basemap',
  'app.recenter': 'Recenter',
  'app.recenterTitle': 'Recenter the map',
  'app.history': 'History',
  'app.historyTitle': 'Overlay the heatmap and recorded trails',
  'app.theatre': 'Theatre',
  'app.theatreTitle': 'DCS theatre: frames the map and lists its airfields',
  'app.bounds': 'Bounds',
  'app.boundsTitle': 'Outline the DCS map extent',
  'app.airfields': 'Airfields',
  'app.airfieldsTitle': 'Show the theatre airfields on the map (click one for its data)',

  'tab.map': 'Map',
  'tab.session': 'Session',
  'tab.debriefs': 'Debriefs',
  'tab.stats': 'Statistics',
  'tab.analytics': 'Analysis',
  'tab.aerodromes': 'Airfields',
  'tab.missionHistory': 'Mission history',

  'units.one': 'unit',
  'units.many': 'units',
  'units.tracked': 'tracked',
  'units.shown': 'shown',
  'units.updated': 'updated',

  'filters.title': 'Filters',
  'filters.search': 'Search (type, name, country)…',
  'filters.coalitions': 'Coalitions',
  'filters.categories': 'Categories',
  'filters.ownshipOnly': 'My aircraft only',
  'filters.showTrails': 'Show flight trails',
  'filters.reset': 'Reset filters',
  'filters.units': 'Units',
  'filters.none': 'No unit',
  'filters.more': 'more…',

  'coalition.blue': 'Blue',
  'coalition.red': 'Red',
  'coalition.neutral': 'Neutral',
  'coalition.spectator': 'Spectator',

  'category.plane': 'Aircraft',
  'category.heli': 'Helicopters',
  'category.ground': 'Ground',
  'category.ship': 'Ships',
  'category.structure': 'Structures',
  'category.other': 'Other',

  'category.plane.one': 'Aircraft',
  'category.heli.one': 'Helicopter',
  'category.ground.one': 'Vehicle',
  'category.ship.one': 'Ship',
  'category.structure.one': 'Structure',
  'category.other.one': 'Object',

  'unit.type': 'Type',
  'unit.category': 'Category',
  'unit.coalition': 'Coalition',
  'unit.country': 'Country',
  'unit.latitude': 'Latitude',
  'unit.longitude': 'Longitude',
  'unit.altitude': 'Altitude',
  'unit.heading': 'Heading',
  'unit.updated': 'Updated',
  'unit.ownship': 'My aircraft',
  'unit.role': 'Role',
  'unit.close': 'Close',

  'players.title': 'Players',
  'players.none': 'No player connected',
  'players.pilot': 'Pilot',
  'players.score': 'Score',
  'players.kills': 'A/C-G-S',
  'players.landings': 'Ldg',
  'players.ping': 'Ping',
  'players.killsTitle': 'Kills air / ground / ship',
  'players.pingTitle': 'Ping (ms)',

  'events.title': 'Events',
  'events.none': 'No event',
  'events.all': 'All',
  'events.kills': 'Kills',
  'events.friendlyFire': 'Friendly fire',
  'events.crashes': 'Crashes',
  'events.ejections': 'Ejections',
  'events.takeoffs': 'Takeoffs',
  'events.landings': 'Landings',
  'events.deaths': 'Deaths',
  'events.slots': 'Slots',
  'events.connections': 'Connections',
  'events.disconnections': 'Disconnections',
  'events.killedBy': '#{killer} destroyed #{victim} ({weapon})',
  'events.friendlyFired': '#{actor} → friendly #{victim} ({weapon})',
  'events.crashed': '#{pilot} crashed (unit {unit})',
  'events.ejected': '#{pilot} ejected (unit {unit})',
  'events.died': '#{pilot} died (unit {unit})',
  'events.tookOff': '#{pilot} took off from {place}',
  'events.landed': '#{pilot} landed at {place}',
  'events.changedSlot': '#{pilot} changed slot',
  'events.connected': '{name} connected',
  'events.disconnected': '{name} disconnected',
  'events.missionEnded': 'Mission ended — winner: {winner}',

  'chat.title': 'Chat',
  'chat.none': 'No message',
  'chat.placeholder': 'Message…',
  'chat.send': 'Send',
  'chat.system': 'system',
  'chat.unavailable': 'Sending to DCS is not available yet (command channel pending)',
  'chat.unreachable': 'Backend unreachable',
  'chat.refused': 'Send refused ({status})',

  'debriefs.title': 'Debriefs',
  'debriefs.refresh': 'Refresh',
  'debriefs.none': 'No debrief recorded. At the end of a mission, Hooks/dcsmm.lua sends debrief.log to the backend.',
  'debriefs.timeline': 'Timeline',
  'debriefs.pilots': 'Pilots',
  'debriefs.takeoffs': 'Takeoffs',
  'debriefs.landings': 'Landings',
  'debriefs.kills': 'Kills',
  'debriefs.crashes': 'Crashes',
  'debriefs.ejections': 'Ejections',
  'debriefs.duration': 'Duration',
  'debriefs.select': 'Select a debrief to see its timeline.',
  'debriefs.loading': 'Loading…',
  'debriefs.missionEnd': 'Mission end — {comment}',
  'debriefs.label': 'Debrief #{id}',
  'debriefs.killed': '{pilot} destroyed {target} ({weapon})',
  'debriefs.crashed': '{pilot} crashed',
  'debriefs.ejected': '{pilot} ejected',
  'debriefs.died': '{pilot} died',
  'debriefs.engineShutdown': '{pilot} shut down the engines at {place}',

  'stats.title': 'Statistics',
  'stats.career': 'Career',
  'stats.mission': 'Mission',
  'stats.refresh': 'Refresh',
  'stats.missions': 'Missions',
  'stats.pilots': 'Pilots',
  'stats.weapons': 'Weapons',
  'stats.engines': 'Airframes',
  'stats.balance': 'Balance',
  'stats.network': 'Network',
  'stats.noPilot': 'No pilot data.',
  'stats.noWeapon': 'No weapon data. Weapons come from kill events.',
  'stats.noEngine': 'No airframe in this category.',
  'stats.noCoalition': 'No coalition data.',
  'stats.noNetwork': 'No network data.',
  'stats.killsCol': 'Kills',
  'stats.deaths': 'Deaths',
  'stats.landings': 'Ldg',
  'stats.ejections': 'Eject.',
  'stats.crashes': 'Crash',
  'stats.friendlyFire': 'FF',
  'stats.ping': 'Ping',
  'stats.weapon': 'Weapon',
  'stats.targets': 'Targets',
  'stats.type': 'DCS type',
  'stats.losses': 'Losses',
  'stats.sorties': 'Sorties',
  'stats.samples': 'Samples',
  'stats.avgPing': 'Avg ping',
  'stats.maxPing': 'Max ping',
  'stats.points': 'pts',

  'analytics.title': 'Analysis',
  'analytics.heatmap': 'Heatmap',
  'analytics.traffic': 'Traffic',
  'analytics.losses': 'Losses',
  'analytics.hint': 'Aggregation of tracked positions. “Losses” highlights where units disappeared; “Traffic” shows where activity concentrates. Use the History button in the header to overlay the heatmap and trails on the map.',
  'analytics.sortie': 'Sortie analysis',
  'analytics.noTrack': 'No track recorded yet. Positions are sampled during the mission.',
  'analytics.unit': 'Unit',
  'analytics.duration': 'Duration',
  'analytics.distance': 'Distance',
  'analytics.maxAlt': 'Max alt',
  'analytics.maxSpeed': 'Max speed',
  'analytics.maxG': 'Max G',
  'analytics.points': 'Points',

  'aerodromes.title': 'Airfields',
  'aerodromes.refresh': 'Refresh',
  'aerodromes.search': 'Search (name, ICAO code, TACAN)…',
  'aerodromes.nearest': 'Near me',
  'aerodromes.byDistance': 'By distance',
  'aerodromes.onMap': 'On the map',
  'aerodromes.none': 'No airfield',
  'aerodromes.select': 'Select an airfield to see its frequencies.',
  'aerodromes.coalition': 'Coalition',
  'aerodromes.coordinates': 'Coordinates',
  'aerodromes.elevation': 'Elevation',
  'aerodromes.runway': 'Runway',
  'aerodromes.tower': 'Tower',
  'aerodromes.charts': 'Available charts',
  'aerodromes.chartsHint': 'Charts live in maps_dcs/ (not embedded in the binary).',
  'aerodromes.clickHint': 'Click a marker on the map to see its data.',
  'aerodromes.showOnMap': 'Show on the map',

  'visibility.prefix': 'Visibility',
  'visibility.mode.map_only': 'Map only',
  'visibility.mode.my_aircraft': 'My aircraft',
  'visibility.mode.allies': 'Fog of war (allies)',
  'visibility.mode.only_allies': 'Allies only',
  'visibility.mode.all': 'All',
  'visibility.mode.unknown': 'Unknown',
  'visibility.note.override': 'filtering disabled: all units are broadcast',
  'visibility.note.allies': 'sensor-detected contacts are not reproduced (restrictive)',
  'visibility.note.unknown': 'mission options not received yet; restrictive filtering applied',
  'error.aerodromes': 'Airfields unavailable: {detail}',
  'error.debrief': 'Debrief unavailable: {detail}',
  'error.debriefList': 'List unavailable: {detail}',
  'error.stats': 'Statistics unavailable: {detail}',
  'error.analytics': 'Analysis unavailable: {detail}',
  'error.heatmap': 'Heatmap unavailable: {detail}',
};

const fr = {
  'app.title': 'DCS Mission Manager',
  'app.connected': 'connecté',
  'app.offline': 'hors ligne',
  'app.basemap': 'Fond',
  'app.recenter': 'Recentrer',
  'app.recenterTitle': 'Recentrer la carte',
  'app.history': 'Historique',
  'app.historyTitle': 'Superposer la carte de chaleur et les traces enregistrées',
  'app.theatre': 'Théâtre',
  'app.theatreTitle': 'Théâtre DCS : cadre la carte et liste ses aérodromes',
  'app.bounds': 'Limites',
  'app.boundsTitle': 'Contour de la carte DCS',
  'app.airfields': 'Aérodromes',
  'app.airfieldsTitle': 'Afficher les aérodromes du théâtre sur la carte (cliquer pour les données)',

  'tab.map': 'Carte',
  'tab.session': 'Session',
  'tab.debriefs': 'Débriefs',
  'tab.stats': 'Statistiques',
  'tab.analytics': 'Analyse',
  'tab.aerodromes': 'Aérodromes',
  'tab.missionHistory': 'Historique des missions',

  'units.one': 'unité',
  'units.many': 'unités',
  'units.tracked': 'suivie',
  'units.shown': 'affichée',
  'units.updated': 'maj',

  'filters.title': 'Filtres',
  'filters.search': 'Rechercher (type, nom, pays)…',
  'filters.coalitions': 'Coalitions',
  'filters.categories': 'Catégories',
  'filters.ownshipOnly': 'Mon appareil uniquement',
  'filters.showTrails': 'Afficher les traces de vol',
  'filters.reset': 'Réinitialiser les filtres',
  'filters.units': 'Unités',
  'filters.none': 'Aucune unité',
  'filters.more': 'autres…',

  'coalition.blue': 'Bleu',
  'coalition.red': 'Rouge',
  'coalition.neutral': 'Neutre',
  'coalition.spectator': 'Spectateur',

  'category.plane': 'Avions',
  'category.heli': 'Hélicoptères',
  'category.ground': 'Sol',
  'category.ship': 'Navires',
  'category.structure': 'Structures',
  'category.other': 'Autres',

  'category.plane.one': 'Avion',
  'category.heli.one': 'Hélicoptère',
  'category.ground.one': 'Véhicule',
  'category.ship.one': 'Navire',
  'category.structure.one': 'Structure',
  'category.other.one': 'Objet',

  'unit.type': 'Type',
  'unit.category': 'Catégorie',
  'unit.coalition': 'Coalition',
  'unit.country': 'Pays',
  'unit.latitude': 'Latitude',
  'unit.longitude': 'Longitude',
  'unit.altitude': 'Altitude',
  'unit.heading': 'Cap',
  'unit.updated': 'Mise à jour',
  'unit.ownship': 'Mon appareil',
  'unit.role': 'Rôle',
  'unit.close': 'Fermer',

  'players.title': 'Joueurs',
  'players.none': 'Aucun joueur connecté',
  'players.pilot': 'Pilote',
  'players.score': 'Score',
  'players.kills': 'A/S/N',
  'players.landings': 'Att.',
  'players.ping': 'Ping',
  'players.killsTitle': 'Kills air / sol / navire',
  'players.pingTitle': 'Ping (ms)',

  'events.title': 'Événements',
  'events.none': 'Aucun événement',
  'events.all': 'Tous',
  'events.kills': 'Kills',
  'events.friendlyFire': 'Friendly fire',
  'events.crashes': 'Crashes',
  'events.ejections': 'Éjections',
  'events.takeoffs': 'Décollages',
  'events.landings': 'Atterrissages',
  'events.deaths': 'Morts',
  'events.slots': 'Slots',
  'events.connections': 'Connexions',
  'events.disconnections': 'Déconnexions',
  'events.killedBy': '#{killer} a détruit #{victim} ({weapon})',
  'events.friendlyFired': '#{actor} → allié #{victim} ({weapon})',
  'events.crashed': '#{pilot} a crashé (unité {unit})',
  'events.ejected': '#{pilot} s\'est éjecté (unité {unit})',
  'events.died': '#{pilot} est mort (unité {unit})',
  'events.tookOff': '#{pilot} a décollé de {place}',
  'events.landed': '#{pilot} a atterri à {place}',
  'events.changedSlot': '#{pilot} a changé de slot',
  'events.connected': '{name} connecté',
  'events.disconnected': '{name} déconnecté',
  'events.missionEnded': 'Mission terminée — gagnant : {winner}',

  'chat.title': 'Chat',
  'chat.none': 'Aucun message',
  'chat.placeholder': 'Message…',
  'chat.send': 'Envoyer',
  'chat.system': 'système',
  'chat.unavailable': "Envoi vers DCS pas encore disponible (canal de commandes à venir)",
  'chat.unreachable': 'Backend injoignable',
  'chat.refused': 'Envoi refusé ({status})',

  'debriefs.title': 'Débriefs',
  'debriefs.refresh': 'Rafraîchir',
  'debriefs.none': "Aucun débrief enregistré. À la fin d'une mission, Hooks/dcsmm.lua envoie debrief.log au backend.",
  'debriefs.timeline': 'Chronologie',
  'debriefs.pilots': 'Pilotes',
  'debriefs.takeoffs': 'Décollages',
  'debriefs.landings': 'Atterrissages',
  'debriefs.kills': 'Kills',
  'debriefs.crashes': 'Crashes',
  'debriefs.ejections': 'Éjections',
  'debriefs.duration': 'Durée',
  'debriefs.select': 'Sélectionne un débrief pour voir sa chronologie.',
  'debriefs.loading': 'Chargement…',
  'debriefs.missionEnd': 'Fin de mission — {comment}',
  'debriefs.label': 'Débrief #{id}',
  'debriefs.killed': '{pilot} a détruit {target} ({weapon})',
  'debriefs.crashed': '{pilot} a crashé',
  'debriefs.ejected': "{pilot} s'est éjecté",
  'debriefs.died': '{pilot} est mort',
  'debriefs.engineShutdown': '{pilot} a coupé les moteurs à {place}',

  'stats.title': 'Statistiques',
  'stats.career': 'Carrière',
  'stats.mission': 'Mission',
  'stats.refresh': 'Rafraîchir',
  'stats.missions': 'Missions',
  'stats.pilots': 'Pilotes',
  'stats.weapons': 'Armes',
  'stats.engines': 'Engins',
  'stats.balance': 'Balance',
  'stats.network': 'Réseau',
  'stats.noPilot': 'Aucune donnée de pilote.',
  'stats.noWeapon': "Aucune donnée d'arme. Les armes proviennent des événements de kill.",
  'stats.noEngine': 'Aucun engin dans cette catégorie.',
  'stats.noCoalition': 'Aucune donnée de coalition.',
  'stats.noNetwork': 'Aucune donnée réseau.',
  'stats.killsCol': 'Kills',
  'stats.deaths': 'Morts',
  'stats.landings': 'Att.',
  'stats.ejections': 'Éject.',
  'stats.crashes': 'Crash',
  'stats.friendlyFire': 'FF',
  'stats.ping': 'Ping',
  'stats.weapon': 'Arme',
  'stats.targets': 'Cibles',
  'stats.type': 'Type DCS',
  'stats.losses': 'Pertes',
  'stats.sorties': 'Sorties',
  'stats.samples': 'Échantillons',
  'stats.avgPing': 'Ping moyen',
  'stats.maxPing': 'Ping max',
  'stats.points': 'pts',

  'analytics.title': 'Analyse',
  'analytics.heatmap': 'Carte de chaleur',
  'analytics.traffic': 'Trafic',
  'analytics.losses': 'Pertes',
  'analytics.hint': "Agrégation des positions suivies. « Pertes » met en évidence les zones où des unités ont disparu ; « Trafic » montre où l'activité se concentre. Utilise le bouton Historique de l'en-tête pour superposer la carte de chaleur et les traces sur la carte.",
  'analytics.sortie': 'Analyse de sortie',
  'analytics.noTrack': "Aucune trace enregistrée pour l'instant. Les positions sont échantillonnées pendant la mission.",
  'analytics.unit': 'Unité',
  'analytics.duration': 'Durée',
  'analytics.distance': 'Distance',
  'analytics.maxAlt': 'Alt. max',
  'analytics.maxSpeed': 'Vit. max',
  'analytics.maxG': 'G max',
  'analytics.points': 'Points',

  'aerodromes.title': 'Aérodromes',
  'aerodromes.refresh': 'Rafraîchir',
  'aerodromes.search': 'Rechercher (nom, code OACI, TACAN)…',
  'aerodromes.nearest': 'Proches de moi',
  'aerodromes.byDistance': 'Par distance',
  'aerodromes.onMap': 'Sur la carte',
  'aerodromes.none': 'Aucun aérodrome',
  'aerodromes.select': 'Sélectionne un aérodrome pour voir ses fréquences.',
  'aerodromes.coalition': 'Coalition',
  'aerodromes.coordinates': 'Coordonnées',
  'aerodromes.elevation': 'Élévation',
  'aerodromes.runway': 'Piste',
  'aerodromes.tower': 'Tower',
  'aerodromes.charts': 'Cartes disponibles',
  'aerodromes.chartsHint': 'Les cartes se trouvent dans maps_dcs/ (non embarquées dans le binaire).',
  'aerodromes.clickHint': 'Clique un marqueur sur la carte pour voir ses données.',
  'aerodromes.showOnMap': 'Voir sur la carte',

  'visibility.prefix': 'Visibilité',
  'visibility.mode.map_only': 'Carte seule',
  'visibility.mode.my_aircraft': 'Mon appareil',
  'visibility.mode.allies': 'Fog of war (alliés)',
  'visibility.mode.only_allies': 'Alliés uniquement',
  'visibility.mode.all': 'Tout',
  'visibility.mode.unknown': 'Inconnu',
  'visibility.note.override': 'filtrage désactivé : toutes les unités sont diffusées',
  'visibility.note.allies': 'les contacts détectés par les capteurs ne sont pas reproduits (restrictif)',
  'visibility.note.unknown': 'options de mission pas encore reçues ; filtrage restrictif appliqué',
  'error.aerodromes': 'Aérodromes indisponibles : {detail}',
  'error.debrief': 'Débrief indisponible : {detail}',
  'error.debriefList': 'Liste indisponible : {detail}',
  'error.stats': 'Statistiques indisponibles : {detail}',
  'error.analytics': 'Analyse indisponible : {detail}',
  'error.heatmap': 'Heatmap indisponible : {detail}',
};

const DICTS = { en, fr };

/**
 * Translate a key, with optional `{placeholder}` substitution.
 * Falls back to the key itself when missing, so gaps are visible but harmless.
 */
export function translate(locale, key, params) {
  const dict = DICTS[locale] ?? DICTS[DEFAULT_LANG];
  let text = dict[key] ?? DICTS[DEFAULT_LANG][key] ?? key;
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      text = text.replaceAll(`{${k}}`, String(v));
    }
  }
  return text;
}

/** Reactive translator: `{$t('tab.map')}`. */
export const t = derived(lang, ($lang) => (key, params) => translate($lang, key, params));

/** Imperative translator for non-reactive code. */
export function tNow(key, params) {
  return translate(get(lang), key, params);
}

export function setLang(id) {
  if (LANGUAGES.some((l) => l.id === id)) lang.set(id);
}

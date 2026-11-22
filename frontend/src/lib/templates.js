/**
 * Built-in checklist templates.
 *
 * They are generic procedures, not manufacturer checklists: every player must
 * adapt them to their aircraft. Templates are localised in the interface and
 * only copied into the database when the player clicks "Use this template".
 */

export const PHASES = [
  'prestart',
  'start',
  'taxi',
  'before_takeoff',
  'takeoff',
  'climb',
  'cruise',
  'descent',
  'approach',
  'landing',
  'after_landing',
  'shutdown',
  'emergency',
];

const tr = (e, f) => ({ en: e, fr: f });

const TEMPLATES = [
  {
    id: 'generic-prestart',
    phase: 'prestart',
    name: tr('Before start', 'Avant démarrage'),
    description: tr('Cockpit preparation before electrical power.', 'Préparation du cockpit avant la mise sous tension.'),
    items: [
      tr('Battery — ON', 'Batterie — MARCHE'),
      tr('Fuel quantity — CHECK', 'Quantité carburant — VÉRIFIER'),
      tr('Oxygen — CHECK', 'Oxygène — VÉRIFIER'),
      tr('Canopy — CLOSED', 'Verrière — FERMÉE'),
      tr('Warning lights — TEST', 'Voyants d’alarme — TESTER'),
      tr('Altimeter — SET', 'Altimètre — RÉGLER'),
    ],
  },
  {
    id: 'generic-start',
    phase: 'start',
    name: tr('Engine start', 'Démarrage moteur'),
    description: tr('Start the engine and confirm the indications.', 'Démarrer le moteur et contrôler les indications.'),
    items: [
      tr('APU — START', 'APU — DÉMARRER'),
      tr('Engine — START', 'Moteur — DÉMARRER'),
      tr('Oil pressure — CHECK', 'Pression d’huile — VÉRIFIER'),
      tr('Generators — ON', 'Générateurs — MARCHE'),
      tr('Anti-ice — AS REQUIRED', 'Antigivrage — SELON BESOIN'),
    ],
  },
  {
    id: 'generic-taxi',
    phase: 'taxi',
    name: tr('Taxi', 'Roulage'),
    description: tr('Move to the holding point.', 'Rejoindre le point d’attente.'),
    items: [
      tr('Brakes — TEST', 'Freins — TESTER'),
      tr('Nose wheel steering — CHECK', 'Direction de roue avant — VÉRIFIER'),
      tr('Instruments — CHECK', 'Instruments — VÉRIFIER'),
      tr('Taxi clearance — OBTAINED', 'Autorisation de roulage — OBTENUE'),
    ],
  },
  {
    id: 'generic-before-takeoff',
    phase: 'before_takeoff',
    name: tr('Before takeoff', 'Avant décollage'),
    description: tr('Final checks on the runway threshold.', 'Derniers contrôles au seuil de piste.'),
    items: [
      tr('Flight controls — FREE AND CORRECT', 'Commandes de vol — LIBRES ET CORRECTES'),
      tr('Trim — SET', 'Trim — RÉGLÉ'),
      tr('Flaps — SET', 'Volets — RÉGLÉS'),
      tr('Canopy — LOCKED', 'Verrière — VERROUILLÉE'),
      tr('Takeoff clearance — OBTAINED', 'Autorisation de décollage — OBTENUE'),
    ],
  },
  {
    id: 'generic-takeoff',
    phase: 'takeoff',
    name: tr('Takeoff', 'Décollage'),
    description: tr('From brake release to gear and flaps up.', 'Du lâcher des freins à la rentrée des volets.'),
    items: [
      tr('Runway — CONFIRMED', 'Piste — CONFIRMÉE'),
      tr('Throttle — MILITARY', 'Manette — MILITAIRE'),
      tr('Rotate — AT Vr', 'Rotation — À Vr'),
      tr('Landing gear — UP', 'Train — RENTRÉ'),
      tr('Flaps — UP', 'Volets — RENTRÉS'),
    ],
  },
  {
    id: 'generic-climb',
    phase: 'climb',
    name: tr('Climb', 'Montée'),
    description: tr('Establish the climb and check the systems.', 'Établir la montée et contrôler les systèmes.'),
    items: [
      tr('Climb power — SET', 'Puissance de montée — RÉGLÉE'),
      tr('Altimeter — SET', 'Altimètre — RÉGLÉ'),
      tr('Oxygen — ON', 'Oxygène — MARCHE'),
      tr('Engine instruments — CHECK', 'Instruments moteur — VÉRIFIER'),
    ],
  },
  {
    id: 'generic-cruise',
    phase: 'cruise',
    name: tr('Cruise', 'Croisière'),
    description: tr('Periodic checks along the route.', 'Contrôles périodiques le long de la route.'),
    items: [
      tr('Waypoint — CONFIRMED', 'Point de route — CONFIRMÉ'),
      tr('Fuel — CHECK', 'Carburant — VÉRIFIER'),
      tr('Systems — CHECK', 'Systèmes — VÉRIFIER'),
    ],
  },
  {
    id: 'generic-descent',
    phase: 'descent',
    name: tr('Descent', 'Descente'),
    description: tr('Prepare the arrival.', 'Préparer l’arrivée.'),
    items: [
      tr('Altimeter — SET', 'Altimètre — RÉGLÉ'),
      tr('Descent clearance — OBTAINED', 'Autorisation de descente — OBTENUE'),
      tr('Speed — MANAGED', 'Vitesse — GÉRÉE'),
      tr('Approach plate — REVIEWED', 'Carte d’approche — REVUE'),
    ],
  },
  {
    id: 'generic-approach',
    phase: 'approach',
    name: tr('Approach', 'Approche'),
    description: tr('Configure for landing.', 'Configurer l’avion pour l’atterrissage.'),
    items: [
      tr('Altimeter — SET', 'Altimètre — RÉGLÉ'),
      tr('Landing gear — DOWN', 'Train — SORTI'),
      tr('Flaps — APPROACH', 'Volets — APPROCHE'),
      tr('Landing clearance — OBTAINED', 'Autorisation d’atterrissage — OBTENUE'),
    ],
  },
  {
    id: 'generic-landing',
    phase: 'landing',
    name: tr('Landing', 'Atterrissage'),
    description: tr('Final and touchdown.', 'Finale et touché des roues.'),
    items: [
      tr('Gear — DOWN AND LOCKED', 'Train — SORTI ET VERROUILLÉ'),
      tr('Flaps — LANDING', 'Volets — ATTERRISSAGE'),
      tr('Speed — ON TARGET', 'Vitesse — SUR LA CIBLE'),
      tr('Touchdown — ON CENTRELINE', 'Toucher — SUR L’AXE'),
    ],
  },
  {
    id: 'generic-after-landing',
    phase: 'after_landing',
    name: tr('After landing', 'Après atterrissage'),
    description: tr('Vacate the runway and taxi in.', 'Dégager la piste et rejoindre le parking.'),
    items: [
      tr('Brakes — AS REQUIRED', 'Freins — SELON BESOIN'),
      tr('Flaps — RETRACT', 'Volets — RENTRER'),
      tr('Transponder — STANDBY', 'Transpondeur — VEILLE'),
      tr('Taxi clearance — OBTAINED', 'Autorisation de roulage — OBTENUE'),
    ],
  },
  {
    id: 'generic-shutdown',
    phase: 'shutdown',
    name: tr('Shutdown', 'Arrêt'),
    description: tr('Secure the aircraft on the parking spot.', 'Sécuriser l’appareil sur le parking.'),
    items: [
      tr('Parking brake — SET', 'Frein de parking — SERRÉ'),
      tr('Engine — SHUTDOWN', 'Moteur — ARRÊTER'),
      tr('Battery — OFF', 'Batterie — ARRÊT'),
      tr('Canopy — OPEN', 'Verrière — OUVRIR'),
    ],
  },
  {
    id: 'generic-emergency',
    phase: 'emergency',
    name: tr('Emergency', 'Urgence'),
    description: tr('Generic immediate actions; adapt to the aircraft.', 'Actions immédiates génériques ; adapter à l’appareil.'),
    items: [
      { ...tr('Emergency — DECLARE', 'Urgence — DÉCLARER'), critical: true },
      { ...tr('Gear — DOWN', 'Train — SORTI'), critical: true },
      tr('Fuel — CHECK', 'Carburant — VÉRIFIER'),
      tr('Alternate — SELECT', 'Dégagement — SÉLECTIONNER'),
    ],
  },
];

/** Templates resolved in one language, ready to duplicate into the store. */
export function builtinTemplates(lang) {
  const pick = (value) => (value && typeof value === 'object' ? value[lang] ?? value.en : value);
  return TEMPLATES.map((template) => ({
    id: template.id,
    phase: template.phase,
    name: pick(template.name),
    description: pick(template.description),
    items: template.items.map((item) => ({
      // Items are either a localised string or { text: localised, critical }.
      text: pick(item.text ?? item),
      response: item.response ? pick(item.response) : '',
      critical: Boolean(item.critical),
    })),
  }));
}

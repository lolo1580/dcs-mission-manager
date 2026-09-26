# Cartes analytiques & analyse de sortie (Phase 4 bis)

## Principe : pas de projection par théâtre

Les modules 4.3 (cartes) et 4.5 (sortie) reposent sur l'**historique des
positions**. Plutôt que d'utiliser les coordonnées internes `x/y` des débriefs
(qui exigent une projection propre à chaque théâtre), le backend échantillonne les
**positions lat/lng** déjà reçues pour la live map. Résultat : **aucun recalage
géographique**, tous les théâtres fonctionnent de la même façon.

## Collecte : `internal/tracker`

Un échantillonneur périodique (`DCSMM_TRACK_INTERVAL`, défaut 3 s) enregistre
pour chaque unité : position, altitude, cap, vitesse, facteur de charge.

- **Traces** : les déplacements, pour reconstituer les trajectoires.
- **Pertes** : quand une unité **disparaît** du monde plus longtemps que
  `DCSMM_TRACK_GRACE` (défaut 15 s), elle est enregistrée comme perte avec sa
  dernière position connue. C'est une approximation honnête de « détruit ou
  désactivé » — DCS n'envoie pas toujours d'événement explicite.
- **Réapparition** : si l'unité revient, elle n'est plus comptée comme perte.

Si aucune mission n'a été annoncée (seul `Export.lua` installé), le tracker crée
automatiquement une mission de session pour ne rien perdre.

## Rétention

`DCSMM_TRACK_RETENTION` (défaut 24 h) : un passage horaire supprime les données
plus anciennes, pour que la base ne grossisse pas indéfiniment sur un serveur qui
tourne en continu.

## Télémétrie avancée (ownship)

`Export.lua` enrichit le message du joueur avec, **si le serveur l'autorise**
(`allow_ownship_export`) : vitesse vraie et indiquée, Mach, incidence, altitude
sol, facteur de charge (`LoGetAccelerationUnits`). Sinon, seules les positions
sont enregistrées — les colonnes vitesse/G restent vides plutôt que fausses.

## API

| Route | Description |
|---|---|
| `GET /api/analytics/heatmap?source=positions\|losses&grid=0.05` | Points de chaleur agrégés (grille en degrés, sans projection) |
| `GET /api/analytics/tracks` | Traces des appareils les plus actifs + stats de sortie |
| `GET /api/analytics/sorties` | Stats de sortie seules |

## Interface

- Nouvel onglet **Analyse** : source de la carte de chaleur (Trafic / Pertes) et
  tableau d'**analyse de sortie** par unité (durée, distance, altitude max,
  vitesse max, G max, nombre de points).
- Bouton **Historique** dans l'en-tête de la carte : superpose la carte de
  chaleur et les traces enregistrées sur la carte en direct.

La distance est calculée par la formule de **haversine** (grand cercle), sur les
points réellement enregistrés.

## Limites assumées

- La détection de perte est **heuristique** (absence prolongée), pas un événement
  DCS. Le couplage avec les événements `crash`/`pilot_death`/`kill` de la Phase 2
  est possible mais volontairement laissé de côté pour ne pas compter deux fois.
- La télémétrie (G, vitesse, incidence) n'est disponible que pour **son propre
  appareil** en multi ; les autres unités n'ont que des positions.
- L'échantillonnage à 3 s suffit pour une trajectoire lisible ; il est
  configurable si besoin de plus de finesse.

## Tests

`internal/tracker` couvre : échantillonnage, détection de perte (et non-duplication),
réapparition, haversine, et l'analyse de sortie (extrêmes et distance).

# Architecture — DCS Mission Manager

## Principe

DCS World ne peut pas tourner dans Docker (Windows uniquement, GPU + rendu requis).
Le projet est donc conçu en **deux composants** qui communiquent par le réseau :

1. **Les scripts Lua côté DCS** (machine Windows) — collectent et envoient les données.
2. **Le manager** (backend Go + Web UI) — reçoit, stocke, agrège et affiche.

Le même manager se déploie en `.exe` Windows ou en image Docker Linux, **sans
modification du code**.

## Flux de données

```
                          WINDOWS (machine A)
┌──────────────────────────────────────────────────────────┐
│ DCS World                                                 │
│                                                           │
│  Scripts/Export.lua        Hooks/dcsmm.lua                │
│   (live map)               (événements/joueurs)           │
│        │                          │                       │
│        │ UDP/JSON                 │ TCP/JSON              │
└────────┼──────────────────────────┼───────────────────────┘
         │                          │
         ▼                          ▼
┌──────────────────────────────────────────────────────────┐
│ Manager (machine A en .exe, ou machine B en Docker)       │
│                                                           │
│  internal/udp  ──► internal/state ──► internal/api        │
│                                        ├─ REST /api/*      │
│                                        ├─ SSE  /api/events │
│                                        └─ Web UI embarquée │
│                                                           │
│  internal/debrief (Phase 3)                               │
│  internal/stats   (Phase 4)                               │
│  internal/theatre (Phase 1, projection)                   │
└──────────────────────────────────────────────────────────┘
```

## Composants du backend

| Package | Rôle |
|---|---|
| `internal/config` | Configuration via variables `DCSMM_*` |
| `internal/udp` | Réception et décodage des datagrammes de télémétrie |
| `internal/state` | Store en mémoire des unités, avec péremption (TTL) |
| `internal/api` | REST, Server-Sent Events, service de l'UI embarquée |
| `internal/theatre` | Projection `lat/lng ↔ coordonnées DCS` (Phase 1) |
| `internal/debrief` | Parseur de `debrief.log` (Phase 3) |
| `internal/stats` | Agrégations statistiques (Phase 4) |

## Frontend

Svelte + Vite + Leaflet. Le build est écrit dans
`backend/internal/api/dist/` (non versionné), puis embarqué dans le binaire Go via
`//go:embed`. Résultat : **un seul exécutable** contient le backend et l'interface.
Si le frontend n'a pas été buildé, une page de repli est servie automatiquement.

## Choix temps réel : SSE plutôt que WebSocket

Pour la Phase 0/1, les mises à jour sont **descendantes uniquement** (serveur → client).
Le **Server-Sent Events** suffit, s'intègre nativement au navigateur (`EventSource`),
et évite la complexité d'un WebSocket. Un WebSocket bidirectionnel pourra être ajouté
en Phase 2 pour le canal de commandes (kick, changement de mission…), mais ce canal
passera par le socket TCP dédié côté Lua, pas par le navigateur.

## Invariants

- DCS reste sur Windows ; le conteneur ne contient que le manager.
- En Docker sur une autre machine, les scripts Lua ciblent l'**IP LAN** du conteneur,
  jamais `127.0.0.1` (qui bouclerait sur l'hôte Windows).
- L'export Lua est **throttlé** (`LuaExportActivityNextEvent`) : on ne bloque jamais
  une frame du simulateur.
- Les scripts Lua fournis sont **additifs** : ne jamais écraser un `Export.lua`
  existant (Tacview, SRS, DCS-BIOS…).

# Marche à suivre — affichage LCD du PZ70

Date : 6 octobre 2026.

> État mis à jour : plusieurs étapes sont maintenant intégrées au code. Consulter [la reprise et la validation des panneaux](integration-panels.fr.md) pour l’état actuel ; le présent document reste le plan de référence.

Ce document prépare l’implémentation. Il complète [le schéma des panneaux](panels-mapping.fr.md), [les fonctions manquantes](fonctions-manquantes.fr.md) et [le rapport de bugs](audit-bugs-general.fr.md).

## État (mis à jour)

Les **prérequis (§1)** et le **rapport commun (§5)** ont été implémentés :

- **§1** — sorties activables indépendamment des commandes (`/api/mappings/outputs`, interrupteur dédié) ; plus de double envoi (priorité `envoi normal`, sinon mode test) ; resynchronisation des sorties **à chaque trame DCS-BIOS** (`Client.OnFrame`, distinct du callback coalescé) ; position du sélecteur PZ70 lue dans l’état complet du panneau (`panelservice.SelectorMode`) ; invalidation du cache à la (dé)connexion.
- **§5** — un **seul** rapport complet par appareil : le pilote (`internal/led`) construit lignes LCD **et** voyants en une fois, écrit seulement si le rapport change, et enregistre le cache après succès. Un futur écrivain séparé ne peut plus effacer l’écran.
- **§3 partiel** — le modèle de profil porte déjà `displays` (mode, ligne, source, export, conversion, unité), validé, avec compatibilité des anciens profils.
- **§6** — l’onglet Panneaux a une section « Affichage PZ70 » : choix mode/ligne/source/export/conversion/unité, **aperçu** (valeur brute → convertie → texte LCD, via `POST /api/display/preview`) et **test matériel** (écrit des nombres connus sur les deux lignes, `POST /api/display/test`), indépendants des commandes.
- **§7** — deux profils de départ LCD, **vérifiés contre le catalogue local** : M-2000C (HDG = `HSI_HDG`, CRS = `HSI_D_NEEDLE`) et F-5E-3 (HDG = `HSI_HDG`, CRS = `HSI_CRS`), en positionnant le sélecteur sur HDG / CRS. Les deux sont des angles **normalisés** (flottant 0..1 exporté en entier 0..65535), donc l’échelle est `360/65535` degrés. Choix motivé : peu d’appareils exportent une valeur *sélectionnée* en entier (F-16/FA-18 l’exportent en texte, non pris en charge). Un ancien fichier sans `displays` est complété en mémoire depuis les profils de départ.

Restent à faire : **§2** (validation du rapport sur le PZ70 réel), **§4** (conversions par avion à confirmer en vol, exports textuels).

## Objectif

Afficher sur le LCD du PZ70 des valeurs réellement exportées par DCS-BIOS pour l’avion actif, en fonction du sélecteur ALT / VS / IAS / HDG / CRS. Conserver les huit voyants de boutons dans le même rapport USB.

Le choix des valeurs doit rester configurable par avion. Ne pas supposer que chaque appareil exporte une altitude, une vitesse ou un cap sélectionnés. Distinguer explicitement une valeur sélectionnée d’une valeur mesurée : aucune substitution silencieuse.

## 1. Fiabiliser les prérequis

- Corriger le mélange des profils lors des chargements concurrents.
- Ajouter un déclenchement de synchronisation après chaque trame DCS-BIOS appliquée ; le callback actuel ne signale que la première trame et les changements de nom d’avion.
- Conserver séparément la position du sélecteur PZ70 et les impulsions des molettes.
- Lire la position initiale du sélecteur depuis l’état complet du panneau, même sans manipulation de l’utilisateur.
- Corriger le sens de rotation de `LCD_WHEEL` et `PITCH_TRIM` avant d’associer réglage et affichage.
- Empêcher le double envoi lorsqu’envoi normal et mode test sont actifs.

L’affichage seul ne doit pas envoyer de commande au cockpit. Prévoir une activation des sorties distincte de l’activation des commandes : le pilote de voyants actuel dépend du commutateur d’envoi.

## 2. Valider le rapport USB sur le matériel

Réutiliser `backend/internal/panel/output.go` comme point de départ, puis vérifier sur le PZ70 réel :

- taille du rapport et mécanisme d’écriture acceptés sous Windows ;
- ordre des chiffres et correspondance des deux lignes de cinq positions ;
- encodage des chiffres, du signe négatif et des positions vides ;
- coexistence des chiffres et des huit voyants dans un rapport ;
- comportement lors d’un débranchement et d’une reconnexion.

**Attention au code actuel :** les lignes sont initialisées à `0xFF`. Une valeur `nil` laisse donc la ligne vide dans le rapport construit ; elle ne préserve pas automatiquement l’ancien affichage. Le commentaire qui suggère une ligne inchangée doit être vérifié et corrigé selon le comportement réel.

Prévoir un test matériel affichant des valeurs connues, par exemple `12345` en haut et `-1234` en bas. Faire confirmer visuellement chaque position avant de connecter des valeurs DCS.

## 3. Ajouter les affectations LCD aux profils

Étendre le profil avec une section dédiée, par exemple `displays`, sans remplacer `bindings` ni `outputs`.

Chaque affectation doit définir au minimum :

| Champ proposé | Rôle |
|---|---|
| Modèle | `pz70` |
| Mode du sélecteur | ALT, VS, IAS, HDG ou CRS |
| Ligne | Supérieure ou inférieure |
| Source | Identifiant du contrôle DCS-BIOS |
| Export | Index de la sortie choisie dans le contrôle |
| Conversion | Facteur et décalage pour une valeur numérique brute |
| Format | Arrondi, largeur, signe et remplissage |
| Unité descriptive | Par exemple pieds, nœuds ou degrés, visible dans l’interface |

Les noms exacts des champs seront arrêtés pendant l’implémentation. Les unités servent à la configuration et ne doivent pas être promises comme texte affichable sur ce LCD numérique.

Compatibilité : un ancien profil sans `displays` doit continuer de fonctionner, sans affichage LCD configuré. Refuser les doublons pour une même ligne et un même mode, les index d’export invalides et les conversions non finies.

## 4. Lire et convertir les exports DCS-BIOS

Créer un lecteur de valeurs partagé ou dédié aux sorties numériques. Pour un export entier :

1. Vérifier la présence des octets nécessaires en mémoire.
2. Lire la valeur selon le format DCS-BIOS.
3. Appliquer le masque et le décalage `shift_by` déclarés.
4. Effectuer la conversion configurée.
5. Arrondir et contrôler les limites de la ligne LCD.

Ne pas traiter automatiquement toute valeur brute comme des pieds, des nœuds ou des degrés. Valider les conversions sur le catalogue et le comportement de l’avion choisi.

Commencer par les exports entiers. Les exports textuels nécessitent notamment de connaître leur longueur, information absente du modèle `biosmeta.Output` actuel : les prendre en charge dans une deuxième étape, avec extension des métadonnées et validation de leur contenu.

Si la source est absente, périmée, incompatible ou hors plage, utiliser un état défini et visible dans l’interface. Pour la première version, privilégier une ligne vide plutôt qu’une valeur trompeuse ou tronquée.

## 5. Unifier l’écriture des LED et du LCD

Construire un état complet par périphérique PZ70 : ligne supérieure, ligne inférieure et masque des huit voyants.

**Un seul composant doit produire le rapport complet.** Le pilote LED actuel envoie un rapport avec les lignes vides ; conserver un écrivain LCD indépendant ferait effacer l’affichage lors d’une mise à jour des voyants.

Déclencher le recalcul sur :

- une trame DCS-BIOS ;
- un changement du sélecteur ;
- une modification du profil ou de l’activation des sorties ;
- un changement d’avion ou une déconnexion DCS-BIOS ;
- une connexion ou reconnexion du panneau.

N’écrire sur l’USB que si le rapport complet change. Enregistrer le rapport comme envoyé seulement après succès ; invalider le cache à la déconnexion pour forcer une réécriture à la reconnexion.

## 6. Ajouter la configuration dans l’interface

Dans Paramètres → Panneaux, ajouter une section « Affichage PZ70 » :

- choix de l’avion et du mode ALT / VS / IAS / HDG / CRS ;
- configuration des deux lignes ;
- recherche d’une source parmi tous les contrôles possédant un export compatible, même s’ils possèdent aussi des entrées ;
- choix de l’export, de la conversion, de l’unité et du format ;
- aperçu de la valeur brute, de la valeur convertie et du résultat LCD ;
- sauvegarde, édition et suppression ;
- test matériel avec des chiffres connus, indépendant des commandes au cockpit.

Afficher explicitement les modes non configurés et les valeurs indisponibles pour l’avion.

## 7. Ajouter des profils de départ progressivement

Choisir un premier avion dont les exports nécessaires sont présents dans le catalogue local DCS-BIOS. Identifier et tester ses sources avant d’ajouter un profil prédéfini.

| Mode | Valeur envisageable, uniquement si exportée |
|---|---|
| ALT | Altitude sélectionnée |
| VS | Vitesse verticale sélectionnée |
| IAS | Vitesse sélectionnée |
| HDG | Cap sélectionné |
| CRS | Course sélectionnée |

L’affectation aux lignes supérieure et inférieure doit être explicite. Ne pas imposer de valeurs par défaut aux autres avions sans validation.

## 8. Vérifier avant livraison

Tests logiciels nécessaires :

- lecture avec masque et décalage, conversion et arrondi ;
- format positif/négatif, zéro, limites et valeur absente ;
- chargement des anciens profils et aller-retour des nouvelles affectations ;
- sélection des modes et invalidation des données de l’ancien avion ;
- conservation simultanée du LCD et des LED ;
- absence d’écriture répétée quand le rapport est identique ;
- nouvelle tentative après échec USB et réinitialisation du cache à la reconnexion ;
- aucune commande cockpit produite par une simple actualisation de l’affichage.

Validation réelle : tester les cinq positions du sélecteur, modifier les valeurs dans le cockpit, vérifier les voyants, interrompre puis reprendre DCS-BIOS et reconnecter le panneau. Faire le premier essai avec l’envoi de commandes désactivé.

## Critères d’acceptation

- Le LCD suit les valeurs du cockpit disponibles pour l’avion configuré.
- Le changement de sélecteur actualise les lignes sans attendre une nouvelle valeur DCS.
- Les LED restent correctes pendant les mises à jour du LCD.
- Une valeur indisponible n’affiche pas une ancienne valeur d’un autre avion.
- Les anciens profils sont conservés.
- L’affichage fonctionne sans activer l’envoi des commandes.
- Les essais matériels sous Windows sont consignés ; les tests simulés seuls ne constituent pas une validation USB.

## Fichiers concernés

| Fichier ou composant | Changement envisagé |
|---|---|
| `backend/internal/mapping/store.go` | Format des profils et validation des affectations LCD |
| `backend/internal/api/mappings.go` | Lecture et sauvegarde de la section LCD sans perdre les autres affectations |
| `backend/internal/biosmeta/catalog.go` | Métadonnées nécessaires à la lecture des exports |
| `backend/internal/dcsbios/client.go` | Signalement des trames et gestion des valeurs disponibles |
| `backend/internal/panelservice/service.go` | État du sélecteur et cycle de connexion des périphériques |
| `backend/internal/panel/output.go` | Format complet des lignes et des voyants |
| `backend/internal/led/led.go` ou composant de sorties commun | Fusion LED/LCD, cache et écriture USB |
| `backend/internal/app/app.go` | Déclencheurs de synchronisation |
| `frontend/src/lib/mappings.js` | Chargement et sauvegarde des affectations LCD |
| `frontend/src/lib/PanelsPanel.svelte` | Formulaire, aperçu et test matériel |
| `frontend/src/lib/i18n.js` | Libellés français et anglais |

## Livraison progressive et retour arrière

Livrer d’abord le test matériel et le rapport commun LED/LCD, ensuite les sources numériques et la configuration, puis un premier profil d’avion validé.

Avant toute évolution du format, conserver une copie du fichier `mappings.json`. L’affichage doit rester désactivable sans supprimer les affectations. Documenter la compatibilité avec l’ancienne version et restaurer la copie du profil si un retour à cette version est nécessaire.

# Configurer les profils PZ55 et PZ70

Mise à jour : 7 octobre 2026. Les fonctions décrites sont implémentées ; les essais sur les panneaux réels et dans DCS restent à effectuer.

La configuration propose maintenant une vue schématique du PZ55 ou du PZ70. Cliquer sur une touche, un voyant ou une ligne LCD choisit son réglage. Les sections Touches / Voyants / LCD / Diagnostic évitent d’afficher tous les formulaires simultanément. Les listes complètes restent accessibles en les dépliant. La recherche accepte l’identifiant, la description et la catégorie DCS-BIOS.

Le [journal d’intégration](cheminement-integration-touches.fr.md) consigne le cheminement, les choix et les vérifications.

## Sauvegarder et modifier

Sept profils de départ sont maintenant disponibles : F/A-18C, F-16C, F-5E-3, Mirage 2000C, A-10C, JF-17 et AV-8B. Leur détail et les restrictions sont dans [la bibliothèque des profils](../profiles/panels/README.fr.md).

Dans Paramètres → Panneaux, choisir l’avion puis attendre le chargement de ses affectations. **Exporter le profil** télécharge un fichier JSON contenant les commandes, les LED et les lignes LCD. Garder ce fichier avant de modifier la configuration.

Chaque affectation possède un bouton de modification. Régler les champs puis enregistrer, ou annuler pour quitter l’édition. Changer la cible d’une affectation remplace l’ancienne ; une cible déjà utilisée est refusée.

Pour `set_state`, **Positions personnalisées** permet de choisir les valeurs de pression/ON et de relâchement/OFF. Les valeurs sont les indices DCS-BIOS, pas les numéros du bouton matériel. L’inversion échange les deux valeurs. Le retour au centre des volets et la désactivation d’un sélecteur restent ignorés. Sans personnalisation, le calcul zéro/maximum est conservé.

**Importer le profil** ouvre un fichier JSON (maximum 1 Mio). L’aperçu indique l’avion d’origine, l’avion actuellement sélectionné et les quantités. **Appliquer l’import** remplace tout le profil de l’avion sélectionné. Le nom de l’avion contenu dans le fichier ne change pas la destination. Pour revenir en arrière, importer la sauvegarde précédente.

L’import ne vérifie pas que les identifiants DCS-BIOS appartiennent au même avion : lors d’un transfert entre avions, adapter les commandes et les sources. L’import n’active ni l’envoi des commandes ni les sorties.

## Molette de réglage du PZ70

Créer une affectation `PZ70 / LCD_WHEEL`, choisir le mode ALT, VS, IAS, HDG ou CRS, puis la commande et son interface. Refaire pour les modes utiles. L’inversion s’applique au sens de rotation.

Une affectation sans mode sert de secours aux modes sans affectation spécifique. Si une affectation spécifique existe, elle seule est utilisée : la commande générale n’est pas envoyée en plus. Sans affectation spécifique ni générale, la molette ne commande rien.

Le trim `PITCH_TRIM` reste indépendant de cette sélection. Le sélecteur choisit séparément les lignes LCD configurées dans la section affichage ; créer une commande de molette ne configure pas les chiffres automatiquement.

## Couleurs des voyants

Sans règle, le fonctionnement existant reste conservé : le premier export de la source allume le voyant lorsqu’il est actif.

Pour un voyant à plusieurs états, ajouter des règles dans l’ordre souhaité. Chaque règle contient une source numérique DCS-BIOS, un index d’export (0 pour le premier), une comparaison, une valeur et une couleur. Les comparaisons proposées sont égal, différent, supérieur, inférieur, supérieur ou égal et inférieur ou égal. Maximum : 16 règles par voyant.

La première règle satisfaite décide de la couleur. Sans règle satisfaite ou sans donnée disponible, le voyant est éteint. Une règle peut choisir explicitement l’extinction. Le PZ55 accepte vert, rouge, jaune et éteint ; les boutons du PZ70 acceptent vert et éteint.

Exemple pédagogique, à adapter aux valeurs réellement exportées par l’avion :

| Ordre | Condition sur l’export de position du train | Couleur |
|---|---|---|
| 1 | Valeur égale à 1 | Vert |
| 2 | Valeur égale à 2 | Rouge |
| Aucune correspondance | Toute autre valeur | Éteint |

Les valeurs 1 et 2 ne sont pas universelles. Consulter les exports de l’avion avant de reproduire cet exemple. Les règles peuvent lire des sources différentes : par exemple verrouillage et déplacement, en plaçant la condition prioritaire en premier.

## Affichage et perte de connexion

Configurer chaque ligne LCD pour chaque mode : source numérique, index, échelle et décalage. La formule est `arrondi(valeur × échelle + décalage)` ; une échelle enregistrée à zéro est interprétée comme 1. L’unité est une indication dans l’interface et ne s’imprime pas sur le LCD numérique.

Les sorties doivent être activées pour actualiser ou effacer LED/LCD. Elles sont indépendantes de l’envoi des commandes. Une surveillance toutes les secondes efface les sorties après que DCS-BIOS est déclaré déconnecté ; l’effacement n’est donc pas instantané dès la perte d’un paquet.

Une source absente ou un mode LCD non configuré laisse la ligne vide. Désactiver les sorties arrête la synchronisation, sans envoyer une extinction explicite. Pour le test des chiffres, désactiver les sorties automatiques afin d’éviter qu’elles remplacent le rapport de test.

## Essais avant utilisation

1. Garder les commandes désactivées et contrôler le journal : ALT et HDG doivent produire chacun leur commande de molette, une seule fois par impulsion.
2. Vérifier le trim, l’auto-throttle ON/OFF et les volets avec retour au centre.
3. Activer uniquement les sorties ; modifier les états dans le cockpit et contrôler les règles LED et les cinq modes LCD.
4. Arrêter DCS-BIOS et vérifier l’effacement après détection de la déconnexion.
5. Exporter puis réimporter le profil sauvegardé et vérifier commandes, règles et LCD.
6. Activer l’envoi des commandes après validation des attributions.

Le détail des corrections, des tests logiciels et du retour arrière se trouve dans [l’intégration des panneaux](integration-panels.fr.md).

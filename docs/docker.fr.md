# Serveur web DCS Manager (Docker)

Le desktop Windows reste le seul composant qui parle à DCS. Le serveur est une
application web multi-utilisateurs optionnelle : il reçoit les missions
terminées d’un ou plusieurs desktops, gère les comptes et les jetons, et sert
les vols, statistiques, plans de vol et check-lists partagés.

Le desktop continue de fonctionner quand le serveur est absent.

## 1. Premier démarrage sur le LAN

```bash
cp .env.example .env
# Modifie .env, au minimum DCSMANAGER_ADMIN_PASSWORD.
docker compose up -d --build
```

Ouvre `http://<ip-du-serveur>:8080`. Le premier compte est créé à partir de
`DCSMANAGER_ADMIN_USER` et `DCSMANAGER_ADMIN_PASSWORD` **uniquement si la table
des utilisateurs est vide**. Aux démarrages suivants, les comptes en base sont
conservés et le mot de passe de l’environnement est ignoré.

La base SQLite, les sessions et les jetons vivent dans `./data` (monté sur
`/data`). C’est le seul dossier à sauvegarder.

Pour un NAS, construis les deux architectures avec Buildx :

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t dcsmanager-web:2.0.0 .
```

Sous Linux, le conteneur tourne avec l’UID 10001 ; crée le dossier avec le bon
propriétaire avant le premier démarrage :

```bash
mkdir -p data
sudo chown -R 10001:10001 data
```

Docker Desktop sous Windows et macOS gère lui-même les permissions du montage.

## 2. Créer un jeton pour le desktop

1. Connecte-toi en administrateur.
2. Ouvre **Mon profil → Jetons desktop** et crée un jeton pour le PC.
3. Copie le jeton immédiatement : il n’est affiché qu’une fois.

Sur le desktop :

1. Ouvre **Paramètres → Synchronisation serveur**.
2. Saisis `http://<ip-du-serveur>:8080` et colle le jeton.
3. Enregistre, puis **Tester la connexion** et **Synchroniser**.

L’identifiant du desktop est généré automatiquement. Renvoyer la même mission
la met à jour au lieu de créer un doublon.

## 3. Comptes

Les administrateurs créent les comptes depuis **Administration** :

- `member` : vols, statistiques, plans et check-lists partagés ;
- `admin` : en plus, les comptes, les jetons et le journal d’audit.

Pas d’auto-inscription ni de récupération par email. Un administrateur
réinitialise un mot de passe depuis la même page ; changer son mot de passe
déconnecte le compte partout et ouvre une nouvelle session au navigateur
courant.

### Récupération du mot de passe

La récupération par email est optionnelle. Renseigne les variables SMTP dans
`.env`, ainsi que l’URL publique utilisée dans le lien :

```env
DCSMANAGER_SMTP_HOST=smtp.example.com
DCSMANAGER_SMTP_PORT=587
DCSMANAGER_SMTP_USER=dcs@example.com
DCSMANAGER_SMTP_PASSWORD=...
DCSMANAGER_SMTP_FROM=DCS Manager <dcs@example.com>
DCSMANAGER_PUBLIC_URL=https://dcs.example.com
```

Le port 587 avec STARTTLS est la valeur par défaut. La page de connexion affiche
alors **Mot de passe oublié ?** ; le lien est valable 30 minutes et ne peut
servir qu’une fois. Sans SMTP, le bouton indique que la récupération par email
n’est pas configurée ; un administrateur peut toujours réinitialiser un mot de
passe depuis **Administration**.

## 4. HTTPS et exposition Internet

Le conteneur parle HTTP en clair. Pour tout dépassement du LAN de confiance,
place-le derrière un reverse proxy qui termine TLS et règle :

```env
DCSMANAGER_SECURE_COOKIES=true
```

Exemple Caddy minimal :

```caddyfile
dcs.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Lance alors compose avec `DCSMANAGER_PORT=127.0.0.1:8080` pour que le port du
conteneur ne soit pas joignable directement. Les sessions utilisent des cookies
`HttpOnly` en `SameSite=Lax`, et chaque mutation vérifie l’en-tête `Origin`.

## 5. Sauvegarde et mise à jour

Un administrateur peut télécharger un instantané zip depuis
**Administration → Sauvegarde du serveur**. Il contient un `dcsmanager.db`
cohérent et un court README.

```bash
# Sauvegarde
docker compose stop
cp -r data data-backup-$(date +%F)
docker compose start

# Mise à jour
git pull
docker compose up -d --build
```

Le schéma de la base est migré automatiquement au démarrage. Teste une
restauration sur une copie avant de compter sur une sauvegarde.

## 6. Limites

- SQLite en mode WAL convient à un serveur de taille escadrille. PostgreSQL
  n’est pas encore pris en charge.
- Une instance Docker sert une seule escadrille. L’hébergement multi-tenant est
  hors périmètre.
- Le serveur ne lit jamais `Saved Games` ni une installation DCS ; seul le
  desktop le fait.
- Les plans de vol et check-lists appartiennent au compte qui les a créés et
  peuvent être privés ou partagés avec l’escadrille. Le desktop et le serveur
  les échangent dans les deux sens ; les modifications concurrentes utilisent la
  **dernière écriture gagnante** et la copie perdante est conservée dans
  **Administration → Conflits de synchronisation**, où elle peut être restaurée
  ou fusionnée champ par champ.
- La suppression d’un document est synchronisée comme un tombstone : il ne
  réapparaît pas depuis un autre desktop.
- Un document partagé est modifiable par son propriétaire et les administrateurs ;
  les autres membres le voient en lecture seule (le serveur refuse la
  modification).

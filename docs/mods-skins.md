# Mods & skins

DCS Manager can manage the mods and liveries installed in
`Saved Games\DCS`. It is a desktop-only tool: the Docker server profile never
exposes these files.

## Mods

Installed mods are read from:

- `Saved Games\DCS\Mods\<category>\<mod>` — enabled;
- `Saved Games\DCS\Mods.disabled\<category>\<mod>` — disabled;
- `Saved Games\DCS\Mods.quarantine\...` — removed, restorable from the Trash tab.

Actions:

- **Enable / disable** moves the folder between `Mods` and `Mods.disabled`.
- **Move to trash** keeps the folder outside the game and adds a manifest so it
  can be restored.
- **Open folder** opens the mod directory in Explorer.

DCS must be closed for every action. The manager never touches the DCS
installation or its official modules.

## Liveries

The manager scans:

- `Saved Games\DCS\Liveries\<Aircraft>\<Livery>`;
- flat liveries directly under `Saved Games\DCS\Liveries\<Livery>`;
- misplaced packs such as
  `Liveries\<pack>\CoreMods\aircraft\<module>\Liveries\<Aircraft>\<Livery>`;
- liveries bundled inside enabled or disabled mods.

It parses `description.lua` for the display name, countries, texture references
and `../` dependencies. A flat livery can be **classified** into the standard
layout. When several liveries share textures, select or move the whole group so
the relative references keep working.

## Import

The import dialog accepts ZIP, 7z and RAR archives. The archive is analysed
first; nothing is written to the game until the install button is pressed.
Archives can contain one or several mods or liveries.

The archive is extracted into a temporary folder, path traversal and symlinks
are refused, and the target is moved into place only after validation. Existing
items are kept in the trash when replaced.

## Safety

- DCS must be closed.
- Only `Saved Games\DCS` is modified.
- Removed content is quarantined, not deleted.
- Password-protected archives are refused.
- Mods are third-party code; keep a backup of important content.

## Backup

The profile backup now offers a **Liveries** category and includes disabled mods
with the **Mods** category.

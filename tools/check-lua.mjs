// Validate the Lua syntax of the DCS-side scripts.
//
// A syntax error in Export.lua is silent in DCS: the script simply never runs and
// the map stays empty, which costs a full game restart to notice. This checks the
// syntax, the required config keys, and that the DCSMANAGER markers still match the
// constants in install.go byte for byte (they are load-bearing for merge and
// uninstall).
//
// Usage: node tools/check-lua.mjs [--strict]
//
// luaparse is looked up in a few places; when it is absent the check is skipped
// with a notice, unless --strict is passed. That keeps it useful locally without
// breaking a build that does not install it.

import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const require = createRequire(import.meta.url);

function loadLuaparse() {
  const candidates = [
    path.join(process.env.TEMP ?? '/tmp', 'lua-check', 'node_modules', 'luaparse'),
    path.join(root, 'node_modules', 'luaparse'),
    path.join(root, 'frontend', 'node_modules', 'luaparse'),
  ];
  for (const c of candidates) {
    try {
      return require(c);
    } catch {
      /* try the next one */
    }
  }
  return null;
}

const luaparse = loadLuaparse();
if (!luaparse) {
  const msg = 'luaparse not found: skipping the Lua syntax check (npm i luaparse)';
  if (process.argv.includes('--strict')) {
    console.error(msg);
    process.exit(1);
  }
  console.log(msg);
  process.exit(0);
}

const files = ['dcs-lua/Export.lua', 'dcs-lua/PanelCommands.lua', 'dcs-lua/Hooks/dcsmanager.lua'];
let failed = false;

for (const rel of files) {
  const full = path.join(root, rel);
  if (!fs.existsSync(full)) {
    console.log(`  MANQUANT  ${rel}`);
    failed = true;
    continue;
  }
  const src = fs.readFileSync(full, 'utf8');
  try {
    // Lua 5.1 matches DCS's interpreter.
    luaparse.parse(src, { luaVersion: '5.1', comments: false, scope: false });
    console.log(`  OK        ${rel}  (${src.split('\n').length} lines)`);
  } catch (e) {
    failed = true;
    console.log(`  ERREUR    ${rel}: ${e.message}`);
  }
}

// The config is real Lua: the DCS-side scripts load it with loadfile(), and the
// backend parses it with the same grammar. A `#` comment (shell/INI style) is a
// syntax error that makes loadfile() reject the whole chunk, so none of the
// settings apply — silently, because the error is swallowed. Parsing it here
// catches that before it reaches a user. The keys must also be present.
const cfgPath = path.join(root, 'dcs-lua/Config/dcsmanager.cfg');
if (fs.existsSync(cfgPath)) {
  const cfg = fs.readFileSync(cfgPath, 'utf8');
  try {
    luaparse.parse(cfg, { luaVersion: '5.1', comments: false, scope: false });
  } catch (e) {
    failed = true;
    console.log(`  ERREUR    dcs-lua/Config/dcsmanager.cfg: ${e.message}`);
  }
  for (const key of ['dcsmanager_host', 'dcsmanager_udp_port', 'dcsmanager_tcp_port', 'dcsmanager_enabled']) {
    if (!cfg.includes(key)) {
      console.log(`  cfg sans ${key}`);
      failed = true;
    }
  }
}

// The markers are written into the user's Export.lua and must match install.go
// exactly, or merge/uninstall cannot find the block.
const expPath = path.join(root, 'dcs-lua/Export.lua');
const goPath = path.join(root, 'backend/internal/install/install.go');
if (fs.existsSync(expPath) && fs.existsSync(goPath)) {
  const exp = fs.readFileSync(expPath, 'utf8');
  const go = fs.readFileSync(goPath, 'utf8');
  const begin = go.match(/"(-- >>> DCSMANAGER-BEGIN[^"]*)"/);
  const end = go.match(/"(-- <<< DCSMANAGER-END <<<)"/);
  if (!begin || !end) {
    console.log('  marqueurs introuvables dans install.go');
    failed = true;
  } else {
    const okBegin = exp.includes(begin[1]);
    const okEnd = exp.includes(end[1]);
    console.log(`  marqueurs : BEGIN=${okBegin} END=${okEnd}`);
    if (!okBegin || !okEnd) failed = true;
  }
}

process.exit(failed ? 1 : 0);

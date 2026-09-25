// Removes previous frontend build output from the Go embed directory while
// preserving `.gitkeep`. This keeps builds clean without letting Vite's
// `emptyOutDir` delete the committed placeholder (which would break
// `//go:embed` on a fresh checkout).
import { rm } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const here = path.dirname(fileURLToPath(import.meta.url));
const dist = path.resolve(here, '../../backend/internal/api/dist');

await rm(path.join(dist, 'assets'), { recursive: true, force: true });
await rm(path.join(dist, 'index.html'), { force: true });

console.log('clean-dist: removed previous build output');

import { readFileSync } from 'fs';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

export function loadCases<T>(filename: string): T {
  const path = resolve(__dirname, '..', '..', 'tests', filename);
  return JSON.parse(readFileSync(path, 'utf-8')) as T;
}

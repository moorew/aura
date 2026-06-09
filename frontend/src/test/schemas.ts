/**
 * Provides the two real local-DB schemas to the sync tests:
 *
 *  - tauriSchemaSql(): the SQL the Tauri desktop build actually applies, parsed
 *    out of src-tauri/src/db.rs (the tauri-plugin-sql migrations). Parsing the
 *    real Rust source — rather than hand-copying — is deliberate: it means a
 *    missing column in a migration (exactly the `roughly_at` bug) shows up as a
 *    failing sync test, not a stale test fixture that silently agrees with itself.
 *
 *  - LOCAL_SCHEMA_SQL: the Capacitor (Android) schema, imported directly.
 */
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { LOCAL_SCHEMA_SQL } from '$lib/tauri/schema';

const here = path.dirname(fileURLToPath(import.meta.url));
const DB_RS = path.resolve(here, '../../src-tauri/src/db.rs');

/**
 * Extract every migration's SQL from db.rs in version order and concatenate.
 * The migrations use raw string literals: sql: r#" ... "#.
 */
export function tauriSchemaSql(): string {
    const src = readFileSync(DB_RS, 'utf8');
    const blocks: string[] = [];
    const re = /sql:\s*r#"([\s\S]*?)"#/g;
    let m: RegExpExecArray | null;
    while ((m = re.exec(src)) !== null) {
        blocks.push(m[1]);
    }
    if (blocks.length === 0) {
        throw new Error('No migrations parsed from db.rs — parser or file changed');
    }
    return blocks.join('\n');
}

export { LOCAL_SCHEMA_SQL };

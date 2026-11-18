// Membuat variabel template tersedia tanpa frontmatter di src/views/**/*.astro. Astro tidak punya variabel implisit, jadi plugin ini
// menyisipkan ke frontmatter setiap template:
//   const { data, entry, site, sites, global, navigation, collection, taxonomy } = Astro.locals;
// Konsekuensi: jangan mendeklarasikan ulang nama-nama itu di template.
import { readFile } from 'node:fs/promises';

const PRE = 'const { data, entry, site, sites, global, navigation, collection, taxonomy } = Astro.locals;';

export default function templateScope() {
  return {
    name: 'template-scope',
    enforce: 'pre',
    // Pakai `load`, bukan `transform`: plugin Astro sudah meng-compile .astro di tahap transform. Astro tidak me-load file
    // utamanya sendiri (return null), jadi di sini file dibaca dan preamble disisipkan sebelum Astro memprosesnya.
    async load(id) {
      if (id.includes('?') || !/\/src\/views\/.*\.astro$/.test(id.replaceAll('\\', '/'))) return; // ?astro&type=... = modul virtual
      const code = await readFile(id, 'utf8');
      return /^---/.test(code) ? code.replace(/^---[^\n]*\n/, (m) => `${m}${PRE}\n`) : `---\n${PRE}\n---\n${code}`;
    },
  };
}

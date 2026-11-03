import fs from 'fs';
const src = fs.readFileSync(process.argv[2], 'utf8').split('\n').filter(l => !/^\s*\/\//.test(l)).join('\n');
const groups = {};
for (const m of src.matchAll(/const (kRestyleGroup\w+) = '([^']+)'/g)) groups[m[1]] = m[2];
const str = s => { // join adjacent dart string literals
  return [...s.matchAll(/'((?:[^'\\]|\\.)*)'/g)].map(x => x[1].replace(/\\'/g, "'")).join('');
};
const styles = [];
for (const m of src.matchAll(/RestyleStyle\(\s*id:\s*'([^']+)',\s*label:\s*'((?:[^'\\]|\\.)*)',\s*group:\s*(\w+),\s*block:\s*([\s\S]*?),\s*\)/g)) {
  styles.push({ id: m[1], label: m[2].replace(/\\'/g, "'"), group: groups[m[3]] ?? m[3], block: str(m[4]) });
}
const constStr = name => { const m = src.match(new RegExp(`const ${name} =([\\s\\S]*?);`)); return str(m[1]).replace(/\$_baseNegative/, '__BASE__'); };
const base = constStr('_baseNegative');
const out = {
  source: 'Restyler packages/tsumiki_shared/lib/catalogs/restyle_styles.dart (sync via scripts/sync-restyler-styles.mjs)',
  medium: { photo: constStr('_photoMedium'), illustration: constStr('_illustrationMedium') },
  negative: { photo: constStr('_photoNegative').replace('__BASE__', base), illustration: constStr('_illustrationNegative').replace('__BASE__', base) },
  groups: [...src.matchAll(/kRestyleGroups = <String>\[([\s\S]*?)\]/g)][0][1].split(',').map(s => groups[s.trim()]).filter(Boolean),
  styles,
};
fs.writeFileSync(process.argv[3], JSON.stringify(out, null, 1));
console.log(styles.length, 'styles', out.groups);

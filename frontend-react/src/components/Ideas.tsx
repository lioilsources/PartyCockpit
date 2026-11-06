import { useCockpit } from '../CockpitContext';
import ui from './ui.module.css';

export function Ideas() {
  const { code, st, api, act } = useCockpit();
  const ideas = [...(st?.ideas ?? [])].reverse();
  const download = () =>
    act(async () => {
      const url = URL.createObjectURL(new Blob([await api.ideasJson()], { type: 'application/json' }));
      Object.assign(document.createElement('a'), { href: url, download: `namety-${code}.json` }).click();
      URL.revokeObjectURL(url);
    });
  return (
    <section className={ui.section}>
      <h2 className={ui.h2}>
        Náměty na webtoon ({ideas.length})<button onClick={() => void download()}>stáhnout JSON</button>
      </h2>
      <ul style={{ paddingLeft: 18, maxHeight: 300, overflow: 'auto', fontSize: '0.92rem' }}>
        {ideas.length ? (
          ideas.map((i) => (
            <li key={i.id} style={{ marginBottom: 6 }}>
              <strong>{i.nickname}:</strong> {i.text}
            </li>
          ))
        ) : (
          <li className="muted">zatím žádné</li>
        )}
      </ul>
    </section>
  );
}

import { useEffect, useState } from 'react';
import { listPacks, type PackInfo } from '@pc/lib/stylemap';
import { plural, recapUrl, type Episode, type SceneKind } from '@pc/lib/types';
import { useCockpit } from '../CockpitContext';
import { SCENES, SHADERS, WEBTOON_LANGS, sceneParams, toggledEpisode } from '../lib/scene';
import { asFlag, asJson, asString, useStoredState } from '../hooks/useStoredState';
import { cx } from '../lib/format';
import ui from './ui.module.css';

export function ScenePanel() {
  const { party, api, act } = useCockpit();
  const [beats, setBeats] = useState(8);
  const [slideStyle, setSlideStyle] = useState('');
  const [shader, setShader] = useState('');
  const [episodes, setEpisodes] = useState<Episode[]>([]);
  // same keys as the Svelte Cockpit (D21)
  const [wtEpisodes, setWtEpisodes] = useStoredState<string[]>('pc.webtoon.episodes', [], episodeList);
  const [wtLang, setWtLang] = useStoredState('pc.webtoon.lang', 'cs', asString);
  const [wtSpeed, setWtSpeed] = useStoredState('pc.webtoon.speed', 60, speedCodec);
  const [packs, setPacks] = useState<PackInfo[]>([]);
  const [pack, setPack] = useStoredState('pc.stylemap', '', asString);
  const [sweepTransition, setSweepTransition] = useStoredState('pc.stylemap.transition', false, asFlag);

  useEffect(() => {
    let live = true;
    listPacks()
      .then((ps) => {
        if (!live) return;
        setPacks(ps);
        setPack((cur) => (ps.some((x) => x.id === cur) ? cur : (ps[0]?.id ?? '')));
      })
      .catch(() => undefined);
    api
      .episodes()
      .then((e) => {
        if (live) setEpisodes(e);
      })
      .catch(() => undefined);
    return () => {
      live = false;
    };
  }, [api, setPack]);

  const setScene = (kind: SceneKind) => act(() => api.scene(kind, sceneParams(kind, { beats, slideStyle, episodes: wtEpisodes, wtLang, wtSpeed, pack, sweepTransition, shader })));

  return (
    <section className={ui.section}>
      <h2 className={ui.h2}>
        Scéna na plátně: <strong>{party.scene.kind}</strong>
      </h2>
      <div className={ui.row}>
        {SCENES.map((s) => (
          <button
            key={s.kind}
            className={cx(party.scene.kind === s.kind && 'on')}
            onClick={() => {
              void setScene(s.kind);
            }}
          >
            {s.label}
          </button>
        ))}
      </div>
      <div className={cx(ui.row, ui.small)}>
        <label>
          přepnout každých{' '}
          <input
            type="number"
            min={2}
            max={64}
            value={beats}
            onChange={(e) => {
              setBeats(e.target.valueAsNumber || 0);
            }}
          />{' '}
          dob
        </label>
        <label>
          styl slideshow{' '}
          <input
            placeholder="(všechny) styleId"
            value={slideStyle}
            onChange={(e) => {
              setSlideStyle(e.target.value);
            }}
          />
        </label>
        <label>
          shader
          <select
            value={shader}
            onChange={(e) => {
              setShader(e.target.value);
            }}
          >
            {SHADERS.map(([id, label]) => (
              <option key={id} value={id}>
                {label}
              </option>
            ))}
          </select>
        </label>
        {packs.length > 0 && (
          <>
            <label>
              StyleMap
              <select
                value={pack}
                onChange={(e) => {
                  setPack(e.target.value);
                }}
              >
                {packs.map((x) => (
                  <option key={x.id} value={x.id}>
                    {x.title} ({x.n})
                  </option>
                ))}
              </select>
            </label>
            <label>
              <input
                type="checkbox"
                checked={sweepTransition}
                onChange={(e) => {
                  setSweepTransition(e.target.checked);
                }}
              />{' '}
              přechod přes StyleMap
            </label>
          </>
        )}
        <a href={recapUrl(party.joinUrl)} target="_blank" rel="noreferrer">
          recap pro hosty ↗
        </a>
      </div>
      <details className={ui.episodes}>
        <summary>
          📖 Webtoon: {wtEpisodes.length ? `${String(wtEpisodes.length)} ${plural(wtEpisodes.length, 'sloupec', 'sloupce', 'sloupců')}` : 'první epizoda'}
        </summary>
        <div className={cx(ui.row, ui.small)}>
          <label>
            jazyk bublin
            <select
              value={wtLang}
              onChange={(e) => {
                setWtLang(e.target.value);
              }}
            >
              {WEBTOON_LANGS.map((l) => (
                <option key={l} value={l}>
                  {l}
                </option>
              ))}
            </select>
          </label>
          <label>
            rychlost{' '}
            <input
              type="number"
              min={10}
              max={400}
              step={10}
              value={wtSpeed}
              onChange={(e) => {
                setWtSpeed(e.target.valueAsNumber || 60);
              }}
            />{' '}
            px/s
          </label>
          <a href={`/studio/${party.code}`} target="_blank" rel="noreferrer">
            stáhnout další ve Studiu ↗
          </a>
        </div>
        <div className={ui.eplist}>
          {[{ id: 'party', label: '🎉 Party epizoda (z galerie)' }, ...episodes.map((e) => ({ id: e.id, label: `${e.story ? `${e.story} · ` : ''}${e.title} (${String(e.n)})` }))].map((e) => (
            <label key={e.id}>
              <input
                type="checkbox"
                checked={wtEpisodes.includes(e.id)}
                onChange={() => {
                  setWtEpisodes((cur) => toggledEpisode(cur, e.id));
                }}
              />{' '}
              {e.label}
            </label>
          ))}
        </div>
      </details>
    </section>
  );
}

const episodeList = asJson<string[]>();
const speedCodec = { parse: (raw: string) => Number(raw) || 60, serialize: (v: number) => String(v) };

import { useEffect, useState } from 'react';
import { listPacks, type PackInfo } from '@pc/lib/stylemap';
import { recapUrl, type Episode, type SceneKind } from '@pc/lib/types';
import { useCockpit } from '../CockpitContext';
import { SCENES, SHADERS, sceneParams } from '../lib/scene';
import { asFlag, asString, useStoredState } from '../hooks/useStoredState';
import { cx } from '../lib/format';
import ui from './ui.module.css';

export function ScenePanel() {
  const { party, api, act } = useCockpit();
  const [beats, setBeats] = useState(8);
  const [slideStyle, setSlideStyle] = useState('');
  const [shader, setShader] = useState('');
  const [episode, setEpisode] = useState('');
  const [episodes, setEpisodes] = useState<Episode[]>([]);
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
        if (!live) return;
        setEpisodes(e);
        setEpisode((cur) => cur || (e[0]?.id ?? ''));
      })
      .catch(() => undefined);
    return () => {
      live = false;
    };
  }, [api, setPack]);

  const setScene = (kind: SceneKind) => act(() => api.scene(kind, sceneParams(kind, { beats, slideStyle, episode, pack, sweepTransition, shader })));

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
        <label>
          epizoda
          <select
            value={episode}
            onChange={(e) => {
              setEpisode(e.target.value);
            }}
          >
            {episodes.map((e) => (
              <option key={e.id} value={e.id}>
                {e.title}
              </option>
            ))}
            <option value="party">🎉 Party epizoda (z galerie)</option>
          </select>
        </label>
      </div>
    </section>
  );
}

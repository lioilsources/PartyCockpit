import { useState, type SubmitEvent } from 'react';
import { useCockpit } from '../CockpitContext';
import { cx } from '../lib/format';
import ui from './ui.module.css';

export function Lottery() {
  const { party, st, api, act } = useCockpit();
  const [count, setCount] = useState(2);
  const [game, setGame] = useState('');
  const players = (st?.guests ?? []).filter((g) => g.wantsToPlay);

  const draw = (e: SubmitEvent) => {
    e.preventDefault();
    void act(() => api.lottery(count, game));
  };
  const endGame = () =>
    act(async () => {
      await api.stopGame();
      await api.scene('blank', { beats: 8 });
    });

  return (
    <section className={ui.section}>
      <h2 className={ui.h2}>Losování hráčů</h2>
      <p className="muted">chtějí hrát: {players.length ? players.map((g) => g.nickname).join(', ') : 'nikdo (losuje se ze všech)'}</p>
      <form className={ui.row} onSubmit={draw}>
        <input
          placeholder="hra (např. Kiriana co-op)"
          value={game}
          onChange={(e) => {
            setGame(e.target.value);
          }}
        />
        <input
          type="number"
          min={1}
          max={8}
          value={count}
          style={{ maxWidth: 70 }}
          onChange={(e) => {
            setCount(e.target.valueAsNumber || 1);
          }}
        />
        <button className="primary">Losovat</button>
      </form>
      <div className={ui.row}>
        <button onClick={() => act(api.startGame)}>⭕ Spustit Kruh s vylosovanými</button>
        {party.scene.kind === 'game' && <button onClick={() => void endGame()}>Konec hry</button>}
      </div>
      <p className={cx('muted', ui.small)}>
        Kruh: každý hráč brání svůj oblouk pálkou z telefonu, 3 životy, vyhrává poslední. Když telefon zmlkne, hraje za něj autopilot.
      </p>
    </section>
  );
}

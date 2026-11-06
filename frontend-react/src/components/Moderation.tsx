import { useMemo } from 'react';
import { fileUrl, type Asset } from '@pc/lib/types';
import { useCockpit } from '../CockpitContext';
import { useNow } from '../hooks/useNow';
import { ago, cx } from '../lib/format';
import { AssetCard } from './AssetCard';
import ui from './ui.module.css';

const MAX_REJECTED = 12;

/** Splits the Cockpit's assets into the moderation buckets. */
function buckets(assets: Asset[]) {
  return {
    pending: assets.filter((a) => a.status === 'pending'),
    processing: assets.filter((a) => a.status === 'processing' && a.kind !== 'live'),
    failed: assets.filter((a) => a.status === 'failed'),
    rejected: assets.filter((a) => a.status === 'rejected').slice(0, MAX_REJECTED),
  };
}

function ModeSwitch() {
  const { party, st, api, act } = useCockpit();
  const hint =
    party.mode === 'relaxed'
      ? st?.config.moderation === 'none'
        ? 'relaxed bez klasifikátoru = stejně ručně'
        : `auto-schválení při NSFW ≤ ${st?.config.autoApproveMax ?? 0}`
      : 'vše ručně';
  return (
    <div className={cx(ui.row, ui.small)}>
      {(['strict', 'relaxed'] as const).map((mode) => (
        <button key={mode} className={cx(party.mode === mode && 'on')} onClick={() => act(() => api.patch({ mode }))}>
          {mode}
        </button>
      ))}
      <span className="muted">{hint}</span>
    </div>
  );
}

export function Moderation() {
  const { code, st, api, act } = useCockpit();
  const now = useNow(5000);
  const { pending, processing, failed, rejected } = useMemo(() => buckets(st?.assets ?? []), [st?.assets]);
  const fileKey = st?.config.fileKey;

  const approve = (a: Asset) => act(() => api.approve(a.id));
  const reject = (a: Asset) => act(() => api.reject(a.id));
  // one by one: the server broadcasts state after each, and order stays the guests' order
  const approveAll = () =>
    act(async () => {
      for (const a of pending) await api.approve(a.id);
    });

  return (
    <section className={ui.section}>
      <h2 className={ui.h2}>
        Ke schválení ({pending.length})
        {pending.length > 1 && (
          <button className="ok" onClick={() => void approveAll()}>
            Schválit vše
          </button>
        )}
      </h2>
      <ModeSwitch />
      <div className={ui.cards}>
        {pending.length ? (
          pending.map((a) => <AssetCard key={a.id} code={code} asset={a} fileKey={fileKey} onApprove={() => void approve(a)} onReject={() => void reject(a)} />)
        ) : (
          <p className="muted">nic nečeká</p>
        )}
      </div>
      {processing.length > 0 && <p className="muted">maluje se: {processing.map((a) => `${a.meta.nickname ?? ''} (${ago(a.createdAt, now)})`).join(', ')}</p>}
      {failed.length > 0 && <p className="err">selhalo: {failed.length}</p>}
      {rejected.length > 0 && (
        <details>
          <summary className="muted">zamítnuté ({rejected.length})</summary>
          <div className={ui.cards}>
            {rejected.map((a) => (
              <div key={a.id} className={ui.card}>
                {a.kind === 'sticker' ? (
                  <div className={ui.sticker} style={{ background: a.meta.color }}>
                    {a.meta.text}
                  </div>
                ) : (
                  <img src={fileUrl(code, a.file, fileKey)} alt="" />
                )}
                <button onClick={() => void approve(a)}>vrátit</button>
              </div>
            ))}
          </div>
        </details>
      )}
    </section>
  );
}

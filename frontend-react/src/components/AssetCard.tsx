import { fileUrl, type Asset } from '@pc/lib/types';
import { cx } from '../lib/format';
import { Clip } from './Clip';
import ui from './ui.module.css';

type Props = { code: string; asset: Asset; fileKey: string | undefined; onApprove: () => void; onReject: () => void };

/** A pending item: the restyled photo or clip (pending files need the Cockpit fileKey, D19), or a note. */
export function AssetCard({ code, asset: a, fileKey, onApprove, onReject }: Props) {
  return (
    <div className={ui.card}>
      {a.kind === 'sticker' ? (
        <div className={ui.sticker} style={{ background: a.meta.color }}>
          {a.meta.text}
        </div>
      ) : (
        <Clip src={fileUrl(code, a.file, fileKey)} />
      )}
      <div className={ui.meta}>
        <strong>{a.meta.nickname}</strong>
        {a.kind === 'dance' && <span>💃 tanec {a.meta.motionLabel}</span>}
        {a.meta.styleLabel && <span>{a.meta.styleLabel}</span>}
        {a.meta.fallback && <span className={ui.warn}>originál (restyle selhal)</span>}
        {a.nsfwScore >= 0 && <span className={cx(a.nsfwScore > 0.15 && ui.warn)}>NSFW {a.nsfwScore.toFixed(2)}</span>}
      </div>
      <div className={ui.row}>
        <button className="ok" onClick={onApprove} aria-label={`schválit ${a.meta.nickname ?? ''}`}>
          ✓
        </button>
        <button className="bad" onClick={onReject} aria-label={`zamítnout ${a.meta.nickname ?? ''}`}>
          ✕
        </button>
      </div>
    </div>
  );
}

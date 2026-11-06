import { useMemo, useState } from 'react';
import { cockpitApi } from './api/cockpitApi';
import { CockpitContext, type CockpitCtx } from './CockpitContext';
import { Header } from './components/Header';
import { Ideas } from './components/Ideas';
import { Login } from './components/Login';
import { Lottery } from './components/Lottery';
import { Moderation } from './components/Moderation';
import { Music } from './components/Music';
import { Preflight } from './components/Preflight';
import { ScenePanel } from './components/ScenePanel';
import { ShowQueue } from './components/ShowQueue';
import { Streams } from './components/Streams';
import { Subtitles } from './components/Subtitles';
import { useAction } from './hooks/useAction';
import { useCockpitAuth } from './hooks/useCockpitAuth';
import { useCockpitSocket } from './hooks/useCockpitSocket';
import { useViewerAttach, ViewerStore } from './hooks/useViewer';
import styles from './Cockpit.module.css';

type SessionProps = { code: string; token: string; replaceToken: (t: string) => void };

/** A checked token: one socket, one WebRTC viewer, the whole operator UI. */
function CockpitSession({ code, token, replaceToken }: SessionProps) {
  const api = useMemo(() => cockpitApi(code, token), [code, token]);
  const [viewer] = useState(() => new ViewerStore());
  const { model, online, send } = useCockpitSocket({ code, token, ticket: api.ticket, onRtc: viewer.handle, onHello: viewer.reset });
  useViewerAttach(viewer, send);
  const { err, setErr, act } = useAction();

  const ctx = useMemo<CockpitCtx | null>(
    () => (model.party ? { code, api, party: model.party, st: model.st, model, online, act, setErr, replaceToken, viewer, send } : null),
    [code, api, model, online, act, setErr, replaceToken, viewer, send],
  );

  // the server sends the whole state on connect; until then there is nothing to operate
  if (!ctx) return <Login code={code} checking err={err} />;

  return (
    <CockpitContext value={ctx}>
      <main className={styles.ck}>
        <Header />
        {err && <p className="err">{err}</p>}
        <ScenePanel />
        <Preflight />
        <div className={styles.grid}>
          <Moderation />
          <ShowQueue />
          <Streams />
          <Music />
          <Subtitles />
          <Lottery />
          <Ideas />
        </div>
      </main>
    </CockpitContext>
  );
}

export function Cockpit({ code }: { code: string }) {
  const auth = useCockpitAuth(code);
  if (auth.status !== 'ok') return <Login code={code} checking={auth.status === 'checking'} err={auth.err} onSubmit={auth.submit} />;
  return <CockpitSession code={code} token={auth.token} replaceToken={auth.replace} />;
}

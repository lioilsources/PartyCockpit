import { Cockpit } from './Cockpit';
import { parseRoute } from './route';

export function App() {
  const route = parseRoute(location.pathname);
  if (route) return <Cockpit code={route.code} />;
  return (
    <main style={{ maxWidth: 520, margin: '15vh auto', padding: '0 16px' }}>
      <h1>Cockpit (React)</h1>
      <p className="muted">
        Otevři <code>/react/cockpit/&lt;kód party&gt;</code>, nebo založ party na <a href="/">úvodní stránce</a>.
      </p>
    </main>
  );
}

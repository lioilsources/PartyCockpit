import { mount } from 'svelte';
import '@fontsource/space-grotesk/400.css';
import '@fontsource/space-grotesk/700.css';
import '@fontsource/caveat/600.css';
import './app.css';
import App from './App.svelte';
import { initI18n } from './lib/i18n.svelte';

// the catalog for the chosen language loads before the first paint
export default initI18n().then(() => mount(App, { target: document.getElementById('app')! }));

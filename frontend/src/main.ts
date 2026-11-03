import { mount } from 'svelte';
import '@fontsource/space-grotesk/400.css';
import '@fontsource/space-grotesk/700.css';
import '@fontsource/caveat/600.css';
import './app.css';
import App from './App.svelte';

export default mount(App, { target: document.getElementById('app')! });

// ShaderInterlude (M7): full-screen fragment shaders between show blocks.
// All take the same uniforms, so the scene can swap them on the beat:
//   uRes     canvas size in px
//   uTime    seconds since the scene started
//   uBeat    1 on a beat, decays to 0 before the next one
//   uEnergy  0..1, audio energy of the stage input (or a beat-derived stand-in)
//   uFade    0..1 fade-in after a swap

export type Shader = { id: string; label: string; src: string };

const head = `precision mediump float;
uniform vec2 uRes; uniform float uTime, uBeat, uEnergy, uFade;
vec3 pal(float t, vec3 a, vec3 b, vec3 c, vec3 d) { return a + b * cos(6.28318 * (c * t + d)); }
vec3 party(float t) { return pal(t, vec3(.55,.35,.6), vec3(.45,.4,.4), vec3(1.,1.,1.), vec3(.0,.33,.67)); }
float hash(vec2 p) { return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453); }
float noise(vec2 p) {
  vec2 i = floor(p), f = fract(p); f = f * f * (3. - 2. * f);
  return mix(mix(hash(i), hash(i + vec2(1,0)), f.x), mix(hash(i + vec2(0,1)), hash(i + vec2(1,1)), f.x), f.y);
}
float fbm(vec2 p) { float v = 0., a = .5; for (int i = 0; i < 5; i++) { v += a * noise(p); p *= 2.03; a *= .5; } return v; }
`;

const out = (expr: string) => `gl_FragColor = vec4((${expr}) * uFade, 1.);`;

export const shaders: Shader[] = [
  {
    id: 'plasma',
    label: 'Plazma',
    src: `${head}
void main() {
  vec2 uv = (gl_FragCoord.xy - .5 * uRes) / uRes.y;
  float t = uTime * .35;
  float v = sin(uv.x * 6. + t) + sin(uv.y * 5. - t * 1.3) + sin((uv.x + uv.y) * 4. + t * .7)
          + sin(length(uv * (3. + 2. * uBeat)) * 7. - t * 2.);
  vec3 c = party(v * .18 + t * .1 + uEnergy * .3);
  c *= .75 + .5 * uBeat;
  ${out('c')}
}`,
  },
  {
    id: 'tunnel',
    label: 'Tunel',
    src: `${head}
void main() {
  vec2 uv = (gl_FragCoord.xy - .5 * uRes) / uRes.y;
  float r = length(uv), a = atan(uv.y, uv.x);
  float z = .3 / max(r, .001) + uTime * (.8 + uEnergy);
  float rings = smoothstep(.45, .5, abs(fract(z) - .5)) ;
  float spokes = smoothstep(.0, .15, abs(sin(a * 6. + uTime * .5)));
  vec3 c = party(z * .05 + sin(a) * .12) * (rings * .9 + .15) * spokes;
  c += vec3(1., .25, .55) * uBeat * smoothstep(.6, 0., r) * .6;
  c *= smoothstep(0., .25, r);
  ${out('c')}
}`,
  },
  {
    id: 'kaleido',
    label: 'Kaleidoskop',
    src: `${head}
void main() {
  vec2 uv = (gl_FragCoord.xy - .5 * uRes) / uRes.y;
  float a = atan(uv.y, uv.x), r = length(uv);
  float seg = 6.2831 / 8.;
  a = abs(mod(a + uTime * .05, seg) - seg * .5);
  vec2 p = vec2(cos(a), sin(a)) * r * (2.2 - uBeat * .4);
  float n = fbm(p * 3. + vec2(uTime * .2, -uTime * .13));
  float edge = smoothstep(.45, .5, n) - smoothstep(.5, .58, n);
  vec3 c = party(n + r * .4 + uTime * .03) * (.35 + n) + vec3(1.) * edge * (.4 + uEnergy);
  c *= smoothstep(1.2, .2, r);
  ${out('c')}
}`,
  },
  {
    id: 'bars',
    label: 'Ekvalizér',
    src: `${head}
void main() {
  vec2 uv = gl_FragCoord.xy / uRes;
  float n = 28.;
  float i = floor(uv.x * n);
  float h = .15 + .55 * noise(vec2(i * .7, uTime * 1.6)) * (.6 + uEnergy) + .25 * uBeat * hash(vec2(i, floor(uTime * 4.)));
  float y = abs(uv.y - .5) * 2.;
  float gap = step(.12, fract(uv.x * n)) * step(fract(uv.x * n), .88);
  float bar = step(y, h) * gap;
  float glow = exp(-8. * max(y - h, 0.)) * gap * .35;
  vec3 c = party(i / n + uTime * .05) * (bar + glow) + vec3(.03, 0., .06);
  ${out('c')}
}`,
  },
  {
    id: 'aurora',
    label: 'Polární záře',
    src: `${head}
void main() {
  vec2 uv = gl_FragCoord.xy / uRes;
  vec2 p = vec2(uv.x * 3., uv.y * 2.);
  vec3 c = vec3(.01, .0, .04);
  for (int k = 0; k < 3; k++) {
    float fk = float(k);
    float wave = fbm(vec2(p.x * .8 + uTime * (.08 + fk * .03), fk * 4.)) ;
    float band = exp(-pow((uv.y - .35 - fk * .15 - wave * .35) * (9. - uEnergy * 3.), 2.));
    c += party(fk * .25 + uv.x * .2 + uTime * .02) * band * (.6 + .5 * uBeat);
  }
  c += vec3(hash(floor(gl_FragCoord.xy / 3.))) * step(.997, hash(floor(gl_FragCoord.xy / 3.) + 1.)) * .8;
  ${out('c')}
}`,
  },
];

export const shaderById = (id: string) => shaders.find((s) => s.id === id);

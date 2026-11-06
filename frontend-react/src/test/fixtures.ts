import type { Asset, CockpitState, Party } from '@pc/lib/types';

export const party = (over: Partial<Party> = {}): Party => ({
  code: 'ABCD',
  name: 'Silvestr',
  mode: 'strict',
  loopMode: 'loop',
  djfyCode: '',
  createdAt: 0,
  endsAt: Date.UTC(2026, 11, 31, 23, 0),
  joinUrl: 'http://party.test/join/ABCD',
  scene: { kind: 'blank', params: {}, startedAt: 0 },
  clients: {},
  iceServers: [],
  autoLive: false,
  ...over,
});

export const sticker = (id: string, nickname: string, over: Partial<Asset> = {}): Asset => ({
  id,
  guestId: 'secret',
  kind: 'sticker',
  meta: { text: 'Na zdraví!', nickname, color: '#ffd84d' },
  nsfwScore: -1,
  status: 'pending',
  createdAt: 0,
  ...over,
});

export const cockpitState = (assets: Asset[] = []): CockpitState => ({
  assets,
  guests: [],
  ideas: [],
  jobs: [],
  clients: { stage: 1, guest: 2 },
  config: { restyle: 'mock', animate: 'mock', moderation: 'none', autoApproveMax: 0.15, djfyUrl: '', subtitles: false, stageKey: 'sk', fileKey: 'fk' },
});

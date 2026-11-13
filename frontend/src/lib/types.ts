export type Scene = { kind: SceneKind; params: Record<string, any>; startedAt: number };
export type SceneKind = 'slideshow' | 'gallery' | 'stickers' | 'nowplaying' | 'webtoon' | 'lottery' | 'blank' | 'pipwall' | 'dance' | 'interlude' | 'recap' | 'stylesweep' | 'game' | 'strips' | 'tributes';

export type Party = {
  code: string;
  name: string;
  mode: 'strict' | 'relaxed';
  loopMode: 'loop' | 'shuffle';
  djfyCode: string;
  createdAt: number;
  endsAt: number;
  joinUrl: string;
  scene: Scene;
  clients: Record<string, number>;
  iceServers: RTCIceServer[];
  /** Party box defaults (D20); absent elsewhere. */
  box?: { scale: number };
  autoLive: boolean;
};

/** Animation presets (backend providers/animate.go). */
export type Motion = { id: string; label: string; engine: 'live' | 'dance' | 'breeze'; seconds: number };

/** Clips are mp4 from the GPU, animated GIF from the mock provider. */
export const isVideo = (file?: string) => !!file && /\.(mp4|webm)$/i.test(file);

/** A phone's live camera (backend api/streams.go). Stage only ever gets live ones. */
export type LiveStream = {
  id: string;
  name: string;
  source: 'browser' | 'mirrorbooth';
  status: 'pending' | 'live' | 'ended';
  online: boolean;
  createdAt: number;
  reason?: string;
};

export type AssetStatus = 'processing' | 'pending' | 'approved' | 'rejected' | 'failed';
export type Asset = {
  id: string;
  guestId: string;
  kind: 'restyle' | 'sticker' | 'upload' | 'live' | 'dance' | 'breeze';
  srcId?: string;
  file?: string;
  meta: {
    text?: string;
    nickname?: string;
    color?: string;
    sheet?: string;
    style?: string;
    styleLabel?: string;
    medium?: string;
    original?: string;
    fallback?: boolean;
    error?: string;
    motion?: string;
    motionLabel?: string;
    photo?: string;
  };
  /** Approved clip that plays in place of this photo: breeze (D23) before the LivePortrait loop (M4). Show queue only. */
  live?: string;
  nsfwScore: number;
  status: AssetStatus;
  createdAt: number;
};

/** One scene of a Studio show (D23); its items live in params (episodes, packs, assets, images). */
export type ShowEntry = { id: string; kind: SceneKind; title: string; params: Record<string, any>; seconds: number };
export type Show = { name: string; entries: ShowEntry[]; loop: boolean; updatedAt: number };
/** The show with its player state (backend api/show.go ShowState). */
export type ShowState = Show & { playing: boolean; index: number; entryId: string; startedAt: number; endsAt: number };

export type Guest = { id: string; nickname: string; wantsToPlay: boolean; createdAt: number };
export type Idea = { id: string; guestId: string; nickname: string; text: string; createdAt: number };

export type Tempo = { bpm: number; source: 'tap' | 'djfy' | 'none'; anchor: number };

// DJfy host snapshot (DJfy lib/features/party/party_cubit.dart buildSnapshot)
export type Track = { id: string; title: string; artist: string; artworkUrl?: string; bpm?: number; source: string };
export type MusicSnapshot = {
  v: number;
  name: string;
  code: string;
  updatedAt: number;
  guests: number;
  isPlaying: boolean;
  nowPlaying: null | (Track & { proposedByName?: string; positionMs: number; durationMs: number });
  queue: { entryId: string; track: Track; proposedByName: string; up: number; down: number; pinned: boolean; blocked: boolean }[];
};

export type GalleryImage = { id: number; sha256: string; prompt: string; styleId: string; url: string; thumb: string };
/** A webtoon episode (D23): a hand-made folder or a Toonfork pack episode. */
export type Episode = {
  id: string;
  title: string;
  story?: string;
  storyId?: string;
  index: number;
  cover?: string;
  langs: string[];
  audience?: string;
  source: 'folder' | 'pack';
  pack?: string;
  n: number;
};
/** Bubble text is drawn by the stage; the panel images carry none. Positions are 0–1. */
export type WtBubble = { kind: 'speech' | 'thought' | 'shout' | 'narration'; text: Record<string, string>; anchor?: { x: number; y: number } };
export type WtSfx = { text: string; style?: string; pos?: { x: number; y: number } };
export type WtPanel = { image: string; loop?: string; w?: number; h?: number; gap?: string; bubbles?: WtBubble[]; sfx?: WtSfx[] };
export type EpisodeDetail = Episode & { panels: WtPanel[] };

export type CockpitState = {
  assets: Asset[];
  guests: Guest[];
  ideas: Idea[];
  jobs: { intent: string; status: string; count: number; avgMs: number }[];
  clients: Record<string, number>;
  config: { restyle: string; animate: string; moderation: string; autoApproveMax: number; djfyUrl: string; subtitles: boolean; stageKey: string; fileKey: string };
};

/** Party file: approved ones for anyone; `key` (Cockpit fileKey) opens pending ones too (D19). */
export const fileUrl = (code: string, file?: string, key?: string) => (file ? `/files/${code}/${file}${key ? `?k=${key}` : ''}` : '');

/** A guest's own file in any state, through their guest endpoint (D19). */
export const ownFileUrl = (code: string, guestId: string, file?: string) => (file ? `/api/parties/${code}/guests/${guestId}/files/${file}` : '');

/** Guest-facing recap page of a party (M7): /join/<code> → /recap/<code>. */
export const recapUrl = (joinUrl: string) => joinUrl.replace('/join/', '/recap/');

/** Czech plural: 1 fotka, 2–4 fotky, 5+ fotek. */
export const plural = (n: number, one: string, few: string, many: string) => (n === 1 ? one : n >= 2 && n <= 4 ? few : many);

export type RecapItem = {
  id: string;
  kind: 'restyle' | 'upload' | 'dance' | 'sticker';
  file?: string;
  live?: string;
  nickname?: string;
  label?: string;
  text?: string;
  color?: string;
};
export type Recap = { name: string; code: string; endsAt: number; purgeAt: number; url: string; items: RecapItem[] };

/** A subtitle update (backend subtitles.Line); one id is updated several times. */
export type SubtitleLine = { id: number; text: string; lang?: string; translation?: string; toLang?: string; final: boolean; done: boolean };

/** Cockpit preflight (M7): live checks of the evening runbook. */
export type Check = { id: string; label: string; status: 'ok' | 'warn' | 'fail' | 'info'; detail: string };

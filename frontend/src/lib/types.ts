export type Scene = { kind: SceneKind; params: Record<string, any>; startedAt: number };
export type SceneKind = 'slideshow' | 'gallery' | 'stickers' | 'nowplaying' | 'webtoon' | 'lottery' | 'blank' | 'pipwall' | 'dance' | 'interlude' | 'recap' | 'stylesweep' | 'game';

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
  autoLive: boolean;
};

/** Animation presets (backend providers/animate.go). */
export type Motion = { id: string; label: string; engine: 'live' | 'dance'; seconds: number };

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
  kind: 'restyle' | 'sticker' | 'upload' | 'live' | 'dance';
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
  /** Approved LivePortrait loop of this photo (show queue only, M4). */
  live?: string;
  nsfwScore: number;
  status: AssetStatus;
  createdAt: number;
};

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
export type Episode = { id: string; title: string; panels: string[] };

export type CockpitState = {
  assets: Asset[];
  guests: Guest[];
  ideas: Idea[];
  jobs: { intent: string; status: string; count: number; avgMs: number }[];
  clients: Record<string, number>;
  config: { restyle: string; animate: string; moderation: string; autoApproveMax: number; djfyUrl: string; subtitles: boolean };
};

export const fileUrl = (code: string, file?: string) => (file ? `/files/${code}/${file}` : '');

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

export type Scene = { kind: SceneKind; params: Record<string, any>; startedAt: number };
export type SceneKind = 'slideshow' | 'gallery' | 'stickers' | 'nowplaying' | 'webtoon' | 'lottery' | 'blank';

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
};

export type AssetStatus = 'processing' | 'pending' | 'approved' | 'rejected' | 'failed';
export type Asset = {
  id: string;
  guestId: string;
  kind: 'restyle' | 'sticker' | 'upload';
  srcId?: string;
  file?: string;
  meta: {
    text?: string;
    nickname?: string;
    color?: string;
    style?: string;
    styleLabel?: string;
    medium?: string;
    original?: string;
    fallback?: boolean;
    error?: string;
  };
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
  config: { restyle: string; moderation: string; autoApproveMax: number; djfyUrl: string };
};

export const fileUrl = (code: string, file?: string) => (file ? `/files/${code}/${file}` : '');

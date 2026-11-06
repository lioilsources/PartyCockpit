import { isVideo } from '@pc/lib/types';

/** A still, an animated GIF or a looping muted clip, whatever the file is. */
export function Clip({ src }: { src: string }) {
  return isVideo(src) ? <video src={src} autoPlay loop muted playsInline /> : <img src={src} alt="" />;
}

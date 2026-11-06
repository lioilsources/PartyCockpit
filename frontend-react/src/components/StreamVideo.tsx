import { useEffect, useRef } from 'react';

/** A live MediaStream: srcObject is a property, not an attribute, so it is set in an effect. */
export function StreamVideo({ media }: { media: MediaStream }) {
  const ref = useRef<HTMLVideoElement>(null);
  useEffect(() => {
    const v = ref.current;
    if (!v) return;
    if (v.srcObject !== media) v.srcObject = media;
    v.play().catch(() => undefined);
  }, [media]);
  return <video ref={ref} autoPlay playsInline muted />;
}

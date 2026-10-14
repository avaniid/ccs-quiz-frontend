import { useEffect, useRef, useState } from "react";

function secondsUntil(deadlineMs) {
  return Math.max(0, Math.ceil((deadlineMs - Date.now()) / 1000));
}

/**
 * Counts down to a fixed deadline and calls onExpire once when it is reached.
 *
 * It is derived from the deadline every tick rather than decremented, so a
 * reload cannot restart it and a throttled background tab cannot slow it down.
 * Nothing runs (and nothing expires) until `enabled` is true, so the timer
 * cannot fire on a screen that never showed the paper.
 */
export default function useTimer(deadlineMs, enabled, onExpire) {
  const onExpireRef = useRef(onExpire);
  onExpireRef.current = onExpire;
  const expiredRef = useRef(false);
  const [remaining, setRemaining] = useState(() =>
    deadlineMs == null ? 0 : secondsUntil(deadlineMs)
  );

  useEffect(() => {
    if (!enabled || deadlineMs == null) return;

    const tick = () => {
      const left = secondsUntil(deadlineMs);
      setRemaining(left);
      if (left <= 0 && !expiredRef.current) {
        expiredRef.current = true;
        onExpireRef.current?.();
      }
    };
    tick();
    const intervalId = setInterval(tick, 1000);
    return () => clearInterval(intervalId);
  }, [deadlineMs, enabled]);

  return remaining;
}

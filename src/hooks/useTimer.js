import { useEffect, useRef } from "react";

export default function useTimer(timeRemaining, setTimeRemaining, onExpire) {
  const onExpireRef = useRef(onExpire);
  onExpireRef.current = onExpire;
  const expiredRef = useRef(false);

  useEffect(() => {
    if (timeRemaining <= 0) {
      if (!expiredRef.current) {
        expiredRef.current = true;
        onExpireRef.current?.();
      }
      return;
    }

    const intervalId = setInterval(() => {
      setTimeRemaining((prev) => Math.max(0, prev - 1));
    }, 1000);

    return () => clearInterval(intervalId);
  }, [setTimeRemaining, timeRemaining <= 0]);

  useEffect(() => {
    if (timeRemaining <= 0 && !expiredRef.current) {
      expiredRef.current = true;
      onExpireRef.current?.();
    }
  }, [timeRemaining]);
}


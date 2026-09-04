import { useEffect, useRef } from "react";

export default function useFullscreenGuard(onViolation) {
  const onViolationRef = useRef(onViolation);
  onViolationRef.current = onViolation;
  const isUnmountingRef = useRef(false);

  useEffect(() => {
    isUnmountingRef.current = false;

    const requestFullscreen = () => {
      if (!document.fullscreenElement && document.documentElement.requestFullscreen) {
        document.documentElement.requestFullscreen().catch((err) => {
          console.warn("Fullscreen request failed:", err);
        });
      }
    };

    requestFullscreen();

    const handleFullscreenChange = () => {
      if (isUnmountingRef.current) return;

      if (!document.fullscreenElement) {
        onViolationRef.current?.("fullscreen-exit");
        requestFullscreen();
      }
    };

    document.addEventListener("fullscreenchange", handleFullscreenChange);

    return () => {
      isUnmountingRef.current = true;
      document.removeEventListener("fullscreenchange", handleFullscreenChange);
      if (document.fullscreenElement && document.exitFullscreen) {
        document.exitFullscreen().catch((err) => {
          console.warn("Exit fullscreen failed:", err);
        });
      }
    };
  }, []);
}


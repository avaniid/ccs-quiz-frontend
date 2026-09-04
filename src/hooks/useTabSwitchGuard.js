import { useEffect, useRef } from "react";

export default function useTabSwitchGuard(onViolation) {
  const onViolationRef = useRef(onViolation);
  onViolationRef.current = onViolation;
  const lastViolationTimeRef = useRef(0);

  useEffect(() => {
    const triggerViolation = (type) => {
      const now = Date.now();
      if (now - lastViolationTimeRef.current < 500) {
        return;
      }
      lastViolationTimeRef.current = now;
      onViolationRef.current?.(type);
    };

    const handleVisibilityChange = () => {
      if (document.hidden) {
        triggerViolation("tab-switch");
      }
    };

    const handleBlur = () => {
      triggerViolation("window-blur");
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    window.addEventListener("blur", handleBlur);

    return () => {
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      window.removeEventListener("blur", handleBlur);
    };
  }, []);
}


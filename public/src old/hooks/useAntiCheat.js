import { useEffect, useRef } from "react";

function useAntiCheat() {
  const flagsRaised = useRef(0);

  useEffect(() => {
    const flagIt = () => {
      flagsRaised.current += 1;
      console.warn("Flag raised. Total:", flagsRaised.current);
    };

    const handleVisibilityChange = () => {
      if (document.hidden) flagIt();
    };

    const handleBlur = () => flagIt();
    const handleFullscreenExit = () => {
      if (!document.fullscreenElement) flagIt();
    };
    const handleCopy = (e) => {
      e.preventDefault();
      flagIt();
    };
    const handlePaste = (e) => {
      e.preventDefault();
      flagIt();
    };
    const handleContextMenu = (e) => {
      e.preventDefault();
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    window.addEventListener("blur", handleBlur);
    document.addEventListener("fullscreenchange", handleFullscreenExit);
    document.addEventListener("copy", handleCopy);
    document.addEventListener("paste", handlePaste);
    document.addEventListener("contextmenu", handleContextMenu);

    return () => {
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      window.removeEventListener("blur", handleBlur);
      document.removeEventListener("fullscreenchange", handleFullscreenExit);
      document.removeEventListener("copy", handleCopy);
      document.removeEventListener("paste", handlePaste);
      document.removeEventListener("contextmenu", handleContextMenu);
    };
  }, []);

  return flagsRaised;
}

export default useAntiCheat;
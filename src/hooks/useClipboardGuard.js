import { useEffect } from "react";

/**
 * Disables copy, cut, paste, right-click context menu, text drag, and keyboard shortcuts
 * (Ctrl+C, Ctrl+V, Ctrl+X, Ctrl+A, Cmd+C, Cmd+V, Insert combos, devtools shortcuts)
 * while the test is active.
 */
export default function useClipboardGuard(enabled = true) {
  useEffect(() => {
    if (!enabled) return;

    // Prevent clipboard write/read
    const preventClipboard = (e) => {
      e.preventDefault();
      e.stopPropagation();
      if (e.clipboardData) {
        e.clipboardData.clearData();
      }
    };

    // Prevent right-click context menu
    const preventContextMenu = (e) => {
      e.preventDefault();
      e.stopPropagation();
    };

    // Prevent dragging text
    const preventDrag = (e) => {
      e.preventDefault();
      e.stopPropagation();
    };

    // Prevent keyboard shortcuts
    const handleKeyDown = (e) => {
      const isCtrlOrCmd = e.ctrlKey || e.metaKey;
      const key = (e.key || "").toLowerCase();

      // Block Ctrl/Cmd + C, V, X, A, U, S, P
      if (
        isCtrlOrCmd &&
        (key === "c" ||
          key === "v" ||
          key === "x" ||
          key === "a" ||
          key === "u" ||
          key === "s" ||
          key === "p")
      ) {
        e.preventDefault();
        e.stopPropagation();
        return false;
      }

      // Windows shortcuts: Ctrl+Insert (copy), Shift+Insert (paste)
      if ((e.ctrlKey && e.key === "Insert") || (e.shiftKey && e.key === "Insert")) {
        e.preventDefault();
        e.stopPropagation();
        return false;
      }

      // DevTools shortcuts (F12, Ctrl+Shift+I, Ctrl+Shift+J, Ctrl+Shift+C)
      if (
        e.key === "F12" ||
        (isCtrlOrCmd && e.shiftKey && (key === "i" || key === "j" || key === "c"))
      ) {
        e.preventDefault();
        e.stopPropagation();
        return false;
      }
    };

    document.addEventListener("copy", preventClipboard, true);
    document.addEventListener("cut", preventClipboard, true);
    document.addEventListener("paste", preventClipboard, true);
    document.addEventListener("contextmenu", preventContextMenu, true);
    document.addEventListener("dragstart", preventDrag, true);
    document.addEventListener("keydown", handleKeyDown, true);

    return () => {
      document.removeEventListener("copy", preventClipboard, true);
      document.removeEventListener("cut", preventClipboard, true);
      document.removeEventListener("paste", preventClipboard, true);
      document.removeEventListener("contextmenu", preventContextMenu, true);
      document.removeEventListener("dragstart", preventDrag, true);
      document.removeEventListener("keydown", handleKeyDown, true);
    };
  }, [enabled]);
}

export default function WarningModal({
  warningCount,
  visible,
  onDismiss,
  violationType,
}) {
  if (!visible) return null;

  const getMessage = (type) => {
    switch (type) {
      case "tab-switch":
        return "You switched away from the test tab. Please stay on this tab for the rest of the test.";
      case "window-blur":
        return "This window lost focus. Keep the test window active and in front.";
      case "fullscreen-exit":
        return "You exited fullscreen. The test must stay in fullscreen mode.";
      case "face-missing":
        return "Your face was not visible to the camera. Stay centered in frame and facing the screen.";
      case "multiple-faces":
        return "More than one face was detected in camera view. Make sure you are alone in frame.";
      case "device-detected":
        return "A phone, tablet, laptop, or other unauthorized device was detected. Remove it from view.";
      case "voice-mismatch":
        return "An unrecognized voice was heard. Only you should be speaking near the microphone.";
      case "camera-lost":
        return "Camera or microphone access was interrupted. Reconnect them to continue.";
      case "camera-blocked":
        return "Your camera is blocked or turned off. Allow it from the browser address bar.";
      case "mic-blocked":
        return "Your microphone is blocked or muted. Allow it from the browser address bar.";
      default:
        return "Suspicious activity was detected. Please follow all test guidelines.";
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-xs">
      <div className="card max-w-md w-full p-6 text-center shadow-2xl border border-red-200">
        <div className="w-14 h-14 rounded-full bg-red-100 text-red-600 flex items-center justify-center mx-auto mb-3 text-2xl font-bold shadow-xs">
          ⚠
        </div>
        
        <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-red-50 border border-red-200 text-red-700 text-xs font-bold uppercase tracking-wider mb-3">
          Flag #{warningCount} Recorded
        </div>

        <h3 className="font-display text-2xl font-bold text-red-600 mb-2">
          Proctoring Flag
        </h3>

        <p className="text-gray-700 mb-3 font-medium text-sm sm:text-base leading-relaxed">
          {getMessage(violationType)}
        </p>

        <p className="text-xs text-gray-500 mb-6 bg-gray-50 p-3 rounded-lg border border-gray-200">
          This flag has been logged for evaluation. Please adhere to the rules and continue your test.
        </p>

        <button
          type="button"
          onClick={onDismiss}
          className="btn-primary w-full cursor-pointer py-3 text-sm font-semibold shadow-sm"
        >
          I Understand & Resume Test
        </button>
      </div>
    </div>
  );
}


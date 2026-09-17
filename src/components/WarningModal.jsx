export default function WarningModal({
  warningCount,
  maxWarnings = 5,
  visible,
  onDismiss,
  violationType,
}) {
  if (!visible) return null;

  const getMessage = (type) => {
    switch (type) {
      case "tab-switch":
        return "You switched away from the test tab. Stay on this tab for the rest of the test.";
      case "window-blur":
        return "This window lost focus. Keep the test window active and in front.";
      case "fullscreen-exit":
        return "You exited fullscreen. The test must stay in fullscreen mode.";
      case "face-missing":
        return "Your face wasn't visible to the camera. Stay centered in frame and facing the screen.";
      case "multiple-faces":
        return "More than one face was seen in the camera. Make sure you're alone in frame.";
      case "device-detected":
        return "A phone, tablet, laptop or similar device was seen in the camera frame. Remove it from view.";
      case "voice-mismatch":
        return "A voice that didn't match your enrolled voice was heard. Only you should be speaking near the mic.";
      case "camera-lost":
        return "Camera or microphone access was lost. Reconnect them to continue the test.";
      default:
        return "Suspicious activity was detected. Please follow the test rules.";
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="card max-w-md w-full p-6 text-center shadow-xl">
        <div className="w-12 h-12 rounded-full bg-red-100 text-red-600 flex items-center justify-center mx-auto mb-4 text-xl font-bold">
          !
        </div>
        <h3 className="font-display text-2xl font-bold text-red-600 mb-2">
          Warning
        </h3>
        <p className="text-gray-700 mb-2 font-medium">
          {getMessage(violationType)}
        </p>
        <p className="text-sm text-gray-500 mb-6">
          {warningCount >= maxWarnings
            ? "Too many violations were recorded. Your test is being auto-submitted."
            : "Repeated violations will result in your test being auto-submitted."}
        </p>
        {warningCount < maxWarnings && (
          <button
            type="button"
            onClick={onDismiss}
            className="btn-primary w-full cursor-pointer"
          >
            Resume Test
          </button>
        )}
      </div>
    </div>
  );
}


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
        return "Tab switching is not allowed during the test.";
      case "window-blur":
        return "Please keep the test window focused.";
      case "fullscreen-exit":
        return "Exiting fullscreen is not permitted during the test.";
      default:
        return "Suspicious activity detected. Please stay on the test window.";
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="card max-w-md w-full p-6 text-center shadow-xl">
        <div className="w-12 h-12 rounded-full bg-red-100 text-red-600 flex items-center justify-center mx-auto mb-4 text-xl font-bold">
          !
        </div>
        <h3 className="font-display text-2xl font-bold text-red-600 mb-2">
          Warning {warningCount}/{maxWarnings}
        </h3>
        <p className="text-gray-700 mb-2 font-medium">
          {getMessage(violationType)}
        </p>
        <p className="text-sm text-gray-500 mb-6">
          {warningCount >= maxWarnings
            ? "Maximum warnings reached. Your test is being auto-submitted."
            : `You will be auto-submitted after ${maxWarnings} warnings.`}
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


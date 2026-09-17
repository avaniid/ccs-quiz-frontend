export default function VoiceEnrollmentOverlay({ visible, secondsLeft, totalSeconds }) {
  if (!visible) return null;

  const progress = Math.min(
    ((totalSeconds - secondsLeft) / totalSeconds) * 100,
    100
  );

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <div className="card max-w-lg w-full p-8 text-center shadow-xl">
        <h2 className="font-display text-xl font-bold text-[var(--ink)] mb-2">
          Voice Enrollment
        </h2>
        <p className="text-sm text-gray-500 mb-6">
          Read the sentence below aloud clearly and naturally.
        </p>
        <p className="italic text-[var(--blue)] font-medium mb-6">
          "I am ready to take the CCS quiz. I understand the rules and I will
          complete this test honestly and on my own."
        </p>
        <div className="font-display text-4xl font-bold text-[var(--ink)] mb-4">
          {secondsLeft}
        </div>
        <div className="w-full h-2 rounded-full bg-[var(--bg)] border border-[var(--border)] overflow-hidden mb-4">
          <div
            className="h-full bg-[var(--blue)] transition-[width] duration-150"
            style={{ width: `${progress}%` }}
          />
        </div>
        <p className="text-xs text-gray-500">
          Your voice creates a local voiceprint used only to verify it's you
          throughout the test. Nothing is uploaded.
        </p>
      </div>
    </div>
  );
}

import { useLocation } from "react-router-dom";

export default function Submitted() {
  const location = useLocation();
  const reason = location.state?.reason || "manual";

  const getHeadingText = () => {
    switch (reason) {
      case "timeout":
        return "Time's up — your test was auto-submitted.";
      case "warnings":
        return "Your test was auto-submitted due to repeated violations.";
      case "manual":
      default:
        return "Your test has been submitted.";
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-6">
      <div className="card max-w-3xl w-full p-8 md:p-12 text-center shadow-xs">
        <h2 className="font-display text-2xl md:text-3xl font-bold mb-3 text-[var(--ink)]">
          {getHeadingText()}
        </h2>
        <p className="text-gray-600 text-base md:text-lg">
          Results will be out soon.
        </p>
      </div>
    </div>
  );
}
import { useEffect, useState, useRef } from "react";
import { useLocation } from "react-router-dom";
import Navbar from "../components/Navbar";

export default function Submitted() {
  const location = useLocation();
  const reason = location.state?.reason || "manual";
  const [secondsLeft, setSecondsLeft] = useState(5);
  const redirectedRef = useRef(false);

  useEffect(() => {
    const timer = setInterval(() => {
      setSecondsLeft((prev) => {
        if (prev <= 1) {
          clearInterval(timer);
          if (!redirectedRef.current) {
            redirectedRef.current = true;
            window.open("https://www.instagram.com/ccs_tiet/", "_blank", "noopener,noreferrer");
          }
          return 0;
        }
        return prev - 1;
      });
    }, 1000);

    return () => clearInterval(timer);
  }, []);

  const isViolationAutoSubmit = reason === "tab-switch" || reason === "fullscreen-exit";

  const getHeadingText = () => {
    switch (reason) {
      case "timeout":
        return "Time's up — your test was auto-submitted.";
      case "tab-switch":
        return "Test Auto-Submitted: Tab change detected.";
      case "fullscreen-exit":
        return "Test Auto-Submitted: Fullscreen mode was exited.";
      case "warnings":
        return "Your test was auto-submitted due to repeated violations.";
      case "manual":
      default:
        return "Your test has been submitted.";
    }
  };

  return (
    <div className="min-h-screen flex flex-col justify-between">
      {/* Top Navbar */}
      <Navbar activePage="Submitted" />

      {/* Main Content */}
      <main className="flex-1 flex items-center justify-center p-6 py-8">
        <div className="card max-w-2xl w-full p-8 sm:p-12 text-center shadow-xs bg-white">
          <div
            className={`w-14 h-14 rounded-full flex items-center justify-center mx-auto mb-4 text-2xl font-bold ${
              isViolationAutoSubmit
                ? "bg-amber-100 text-amber-700"
                : "bg-green-100 text-green-700"
            }`}
          >
            {isViolationAutoSubmit ? "!" : "✓"}
          </div>
          <h2 className="font-display text-2xl sm:text-3xl font-bold mb-3 text-[var(--ink)]">
            {getHeadingText()}
          </h2>
          <p className="text-gray-600 text-sm sm:text-base mb-6">
            Thank you for completing the assessment. Results and interview calls will be communicated soon.
          </p>

          <div className="p-4 rounded-xl bg-[var(--bg)] border border-[var(--border)] inline-flex flex-col items-center gap-2">
            <p className="text-xs sm:text-sm text-gray-600">
              {secondsLeft > 0 ? (
                <>
                  Opening CCS Instagram in a new tab in{" "}
                  <span className="font-bold text-[#3A94C2]">{secondsLeft}s</span>...
                </>
              ) : (
                "Opened CCS Instagram in a new tab."
              )}
            </p>
            <a
              href="https://www.instagram.com/ccs_tiet/"
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs sm:text-sm text-[#3A94C2] font-semibold hover:underline"
            >
              Click here if it didn't open automatically →
            </a>
          </div>
        </div>
      </main>

      <footer className="py-4 shrink-0" />
    </div>
  );
}
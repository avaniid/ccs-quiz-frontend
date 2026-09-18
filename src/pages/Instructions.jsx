import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { verifySession } from "../services/api";

export default function Instructions() {
  const [error, setError] = useState("");
  const navigate = useNavigate();

  // Don't let anyone read the rules into a test they can't start — /quiz/get
  // needs the session cookie this check validates.
  useEffect(() => {
    let isMounted = true;
    verifySession().catch(() => {
      if (isMounted) navigate("/", { replace: true });
    });
    return () => {
      isMounted = false;
    };
  }, [navigate]);

  const handleAccept = async () => {
    // Fullscreen first, and before any await: requestFullscreen needs a live
    // user gesture, and the getUserMedia permission prompt consumes the one
    // from this click. Entering it here also keeps useFullscreenGuard on the
    // quiz page from retrying (and failing) without a gesture of its own.
    const fullscreenRequest = document.fullscreenElement
      ? Promise.resolve()
      : document.documentElement.requestFullscreen?.().catch(() => {});

    try {
      await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
      await fullscreenRequest;
      navigate("/quiz");
    } catch (err) {
      setError("Camera and microphone access is required to start the test.");
    }
  };

  const rules = [
    {
      title: "Stay alone in frame",
      detail:
        "Keep only yourself visible to the camera. If a second face is detected, or your face isn't visible, a warning is raised.",
    },
    {
      title: "No phones, tablets or other devices",
      detail:
        "Don't bring a phone, tablet, laptop, book, or other electronic device into the camera's view — it will be detected and flagged.",
    },
    {
      title: "No other voices",
      detail:
        "You'll record a short voice sample before the test starts. After that, only your voice should be heard — a different voice nearby will raise a warning.",
    },
    {
      title: "Don't look away from the screen",
      detail: "Keep your attention on the test window for the full duration.",
    },
    {
      title: "Don't switch tabs or windows",
      detail: "Switching tabs, minimizing, or clicking outside this window raises a warning.",
    },
    {
      title: "Stay in fullscreen",
      detail: "Exiting fullscreen mode during the test raises a warning.",
    },
    {
      title: "Keep camera & microphone on",
      detail: "Losing camera or microphone access during the test raises a warning.",
    },
  ];

  return (
    <div className="min-h-screen flex items-center justify-center p-6">
      <div className="card max-w-3xl w-full p-8 md:p-10 shadow-xs">
        <h2 className="font-display text-2xl md:text-3xl font-bold mb-2 text-[var(--ink)]">
          Instructions
        </h2>
        <p className="text-sm text-gray-500 mb-6">
          This test is proctored. Please read the rules below carefully before you begin.
        </p>

        <ul className="space-y-3 mb-6">
          {rules.map((rule) => (
            <li
              key={rule.title}
              className="flex gap-3 p-3 rounded-lg border border-[var(--border)] bg-[var(--bg)]"
            >
              <span className="w-2 h-2 mt-1.5 rounded-full bg-[var(--blue)] shrink-0" />
              <div>
                <p className="font-semibold text-sm text-[var(--ink)]">{rule.title}</p>
                <p className="text-sm text-gray-600">{rule.detail}</p>
              </div>
            </li>
          ))}
        </ul>

        <p className="text-sm text-gray-500 mb-8">
          Repeated violations of the rules above can result in your test being
          automatically submitted.
        </p>

        <div className="flex flex-col items-start gap-4">
          <button onClick={handleAccept} className="btn-primary cursor-pointer">
            I Agree — Enable Camera & Mic
          </button>
          {error && <p className="text-red-600 text-sm font-medium">{error}</p>}
        </div>
      </div>
    </div>
  );
}
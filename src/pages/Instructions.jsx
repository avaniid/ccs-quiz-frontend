import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  verifySession,
  logout,
  getGoogleLoginUrl,
} from "../services/api";
import { acquireCameraStream } from "../hooks/cameraStore";
import Navbar from "../components/Navbar";

function describeMediaError(err) {
  switch (err?.name) {
    case "NotAllowedError":
      return "You blocked camera or microphone access. Click the camera icon in your browser's address bar, allow both, then try again.";
    case "NotFoundError":
      return "No camera or microphone was found. Connect one and try again.";
    case "NotReadableError":
      return "Your camera or microphone is being used by another app. Close it (Zoom, Meet, Teams, another tab) and try again.";
    case "SecurityError":
      return "Camera access needs a secure connection. Open this page over https.";
    default:
      return `Camera and microphone access is required to start the test. (${
        err?.name || "unknown error"
      })`;
  }
}

export default function Instructions() {
  const [user, setUser] = useState(null);
  const [loadingUser, setLoadingUser] = useState(true);
  const [error, setError] = useState("");
  const navigate = useNavigate();

  useEffect(() => {
    let isMounted = true;
    verifySession()
      .then((data) => {
        if (isMounted) {
          setUser(data);
          setLoadingUser(false);
        }
      })
      .catch(() => {
        if (isMounted) {
          setUser(null);
          setLoadingUser(false);
        }
      });

    return () => {
      isMounted = false;
    };
  }, []);

  // Any signed-in Google account may continue: the quiz is open to every
  // registrant, not only @thapar.edu. Which accounts may sign in at all is
  // decided by the backend (ALLOWED_EMAIL_DOMAINS), and an account without an
  // assigned paper is stopped with "no quiz assigned" on the next page.
  const isAuthorized = Boolean(user);

  const handleSignIn = () => {
    window.location.href = getGoogleLoginUrl();
  };

  const handleSignOut = async () => {
    await logout();
    setUser(null);
  };

  const handleAccept = async () => {
    setError("");
    try {
      await acquireCameraStream();

      if (!document.fullscreenElement) {
        await document.documentElement.requestFullscreen?.().catch(() => {});
      }
      navigate("/quiz");
    } catch (err) {
      setError(describeMediaError(err));
    }
  };

  return (
    <div className="min-h-screen flex flex-col justify-between">
      {/* Top Navbar */}
      <Navbar activePage="Instructions" />

      {/* Main Container */}
      <main className="flex-1 max-w-4xl w-full mx-auto px-6 py-6 sm:py-8 flex flex-col justify-center">
        <div className="card p-8 sm:p-10 shadow-xs bg-white">
          {/* Header */}
          <div className="border-b border-[var(--border)] pb-6 mb-6">
            <h1 className="font-display text-2xl sm:text-3xl font-bold text-[var(--ink)] mb-2">
              Assessment Instructions
            </h1>
            <p className="text-sm text-gray-500">
              Please review the guidelines below. Camera and microphone permissions will be requested when you begin.
            </p>
          </div>

          {/* Authentication Status Banner */}
          <div className="mb-8">
            {loadingUser ? (
              <div className="p-4 rounded-lg bg-[var(--bg)] border border-[var(--border)] text-sm text-gray-500">
                Checking candidate session...
              </div>
            ) : isAuthorized ? (
              <div className="p-4 rounded-lg bg-green-50/70 border border-green-200 flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-center gap-3">
                  <span className="w-8 h-8 rounded-full bg-green-100 text-green-700 flex items-center justify-center font-bold text-sm shrink-0">
                    ✓
                  </span>
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wider text-green-700">
                      Signed in
                    </p>
                    <p className="text-sm font-medium text-gray-800">
                      {user.email} {user.name ? `(${user.name})` : ""}
                    </p>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={handleSignOut}
                  className="text-xs text-gray-500 hover:text-red-600 underline cursor-pointer"
                >
                  Switch account
                </button>
              </div>
            ) : (
              <div className="p-5 rounded-lg bg-amber-50/80 border border-amber-200 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                <div className="flex items-start gap-3">
                  <span className="w-8 h-8 rounded-full bg-amber-100 text-amber-700 flex items-center justify-center font-bold text-sm shrink-0 mt-0.5">
                    !
                  </span>
                  <div>
                    <p className="text-sm font-semibold text-amber-900">
                      Sign In Required
                    </p>
                    <p className="text-xs text-amber-800 mt-0.5">
                      Sign in with the <span className="font-semibold">same Google account you registered with</span> to unlock the test.
                    </p>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={handleSignIn}
                  className="bg-[#3A94C2] hover:bg-[#317da4] text-white text-xs sm:text-sm font-medium px-5 py-2.5 rounded-full cursor-pointer transition-all shrink-0 shadow-xs"
                >
                  Sign In with Google
                </button>
              </div>
            )}
          </div>

          {/* Exam Rules & Guidelines */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
            {/* Format Column */}
            <div className="p-5 rounded-xl border border-[var(--border)] bg-[var(--bg)]/60">
              <h3 className="font-display font-bold text-base text-[var(--ink)] mb-3 flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-[#3A94C2]" />
                Format & Navigation
              </h3>
              <ul className="text-xs sm:text-sm text-gray-600 space-y-2.5">
                <li>• 10 questions covering aptitude and technical core.</li>
                <li>• 30-minute strict time window with automated timer.</li>
                <li>• Palette navigation: mark questions for review and return at any time.</li>
                <li>• Responses can be modified until final submission.</li>
              </ul>
            </div>

            {/* Proctoring Column */}
            <div className="p-5 rounded-xl border border-[var(--border)] bg-[var(--bg)]/60">
              <h3 className="font-display font-bold text-base text-[var(--ink)] mb-3 flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-[#3A94C2]" />
                Proctoring Standards
              </h3>
              <ul className="text-xs sm:text-sm text-gray-600 space-y-2.5">
                <li>• Camera & mic must remain active in fullscreen mode.</li>
                <li>• Switching tabs or losing window focus triggers violation warnings.</li>
                <li>• Reaching 5 proctoring strikes auto-submits your attempt immediately.</li>
                <li>• Maintain a quiet, well-lit environment throughout.</li>
              </ul>
            </div>
          </div>

          {/* Action Footer */}
          <div className="border-t border-[var(--border)] pt-6 flex flex-col items-start gap-4">
            {isAuthorized ? (
              <>
                <button
                  onClick={handleAccept}
                  className="bg-[#3A94C2] hover:bg-[#317da4] text-white font-medium text-sm sm:text-base px-8 py-3 rounded-full cursor-pointer transition-all shadow-xs hover:shadow-md"
                >
                  I Agree — Enable Camera & Mic
                </button>
                {error && <p className="text-red-600 text-xs sm:text-sm font-medium">{error}</p>}
              </>
            ) : (
              <div className="text-xs sm:text-sm text-gray-500 font-normal">
                Please sign in above with the Google account you registered with to proceed.
              </div>
            )}
          </div>
        </div>
      </main>

      {/* Bottom spacer */}
      <footer className="py-6" />
    </div>
  );
}
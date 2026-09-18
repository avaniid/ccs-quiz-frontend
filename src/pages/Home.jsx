import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { getGoogleLoginUrl, verifySession } from "../services/api";

export default function Home() {
  const navigate = useNavigate();
  const [checkingSession, setCheckingSession] = useState(true);

  // Skip the sign-in step if the session cookie is still valid.
  useEffect(() => {
    let isMounted = true;
    verifySession()
      .then(() => {
        if (isMounted) navigate("/instructions", { replace: true });
      })
      .catch(() => {
        if (isMounted) setCheckingSession(false);
      });
    return () => {
      isMounted = false;
    };
  }, [navigate]);

  const handleLogin = () => {
    // Full page navigation, not XHR: the OAuth handshake sets cookies and
    // bounces through Google before landing back on /login-redirect.
    window.location.href = getGoogleLoginUrl();
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-6">
      <div className="card max-w-3xl w-full p-8 md:p-12 text-center shadow-xs">
        <h1 className="font-display text-3xl md:text-4xl font-bold mb-4 text-[var(--ink)]">
          CCS Quiz Portal
        </h1>
        <p className="text-gray-600 mb-8 text-base md:text-lg">
          Welcome to the CCS Quiz Portal. Please sign in to begin your assessment.
        </p>
        <button
          onClick={handleLogin}
          disabled={checkingSession}
          className="btn-primary cursor-pointer disabled:opacity-60 disabled:cursor-not-allowed"
        >
          {checkingSession ? "Checking session..." : "Sign in with Google"}
        </button>
      </div>
    </div>
  );
}
import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { getGoogleLoginUrl, verifySession } from "../services/api";
import Navbar from "../components/Navbar";
import { FlippingWordSwap } from "../components/ui/FlippingWordSwap";

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
    <div className="min-h-screen flex flex-col justify-between">
      {/* --- TOP NAVBAR WITH QUICK ACCESS LINKS RIBBON --- */}
      <Navbar activePage="Home" />

      {/* --- HERO SECTION --- */}
      <main className="flex-1 flex flex-col items-center justify-center px-6 py-8 text-center">
        {/* 
          TINKER: HERO TEXT SIZE & FONT
          - Font sizes: text-3xl, text-4xl, text-5xl, text-6xl, text-7xl
          - Font family: font-display (Space Grotesk) or font-sans
          - Weight: font-bold, font-semibold, font-medium
          - Letter tracking: tracking-tight, tracking-normal, tracking-wide
          - Line height: leading-tight, leading-snug, leading-normal
        */}
        <div className="mb-6 max-w-4xl w-full mx-auto flex items-center justify-center">
          <FlippingWordSwap
            word1="Want to join CCS?"
            word2="Welcome To The CCS Quiz Portal"
            duration={450}
            stagger={25}
            autoSwap={true}
            autoSwapDelay={1800}
            className="font-display text-4xl sm:text-5xl md:text-6xl font-bold text-[var(--ink)] tracking-tight text-center leading-tight"
            toClassName="font-display text-4xl sm:text-5xl md:text-6xl font-bold text-[var(--ink)] tracking-tight text-center leading-tight"
          />
        </div>

        {/* 
          TINKER: CTA BUTTON & SUBTITLE
          - Button text size: text-xs, text-sm, text-base, text-lg
          - Button padding: px-8 py-2.5 (horizontal/vertical)
          - Button radius: rounded-full (pill) or rounded-lg / rounded-md
          - Button color: bg-[#3A94C2] (or any hex code)
        */}
        <div className="flex flex-col items-center gap-2">
          <button
            onClick={handleLogin}
            disabled={checkingSession}
            className="bg-[#3A94C2] hover:bg-[#317da4] active:scale-[0.98] text-white text-sm sm:text-base font-medium px-8 py-2.5 rounded-full transition-all duration-150 shadow-xs hover:shadow-md cursor-pointer disabled:opacity-60 disabled:cursor-not-allowed"
          >
            {checkingSession ? "Checking session..." : "sign in to continue"}
          </button>
          
          {/* TINKER: Subtitle text size (text-xs, text-sm, text-base) */}
          <p className="text-gray-500 text-xs sm:text-sm font-normal">
            Use the Google account you registered with
          </p>
        </div>
      </main>

      {/* --- FOOTER SPACER (keeps center perfectly balanced) --- */}
      <footer className="py-4 shrink-0" />
    </div>
  );
}
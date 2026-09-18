import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { verifySession, checkAlreadySubmitted } from "../services/api";

/**
 * Landing page for the backend's OAuth callback (FRONTEND_REDIRECT_URL must
 * point here). It confirms the session cookie actually made it across before
 * letting the candidate into the test.
 */
function LoginRedirect() {
  const navigate = useNavigate();
  const [message, setMessage] = useState("Verifying your login...");

  useEffect(() => {
    let isMounted = true;

    async function verify() {
      try {
        await verifySession();
      } catch {
        if (isMounted) navigate("/", { replace: true }); // not actually logged in
        return;
      }

      // Someone returning after finishing should land on the thank-you page,
      // not back inside a test they can no longer submit.
      try {
        if (await checkAlreadySubmitted()) {
          if (isMounted) navigate("/submitted", { replace: true });
          return;
        }
      } catch {
        // Non-fatal: fall through to the instructions.
      }

      if (isMounted) navigate("/instructions", { replace: true });
    }

    verify();
    const slowTimer = setTimeout(() => {
      if (isMounted) setMessage("Still verifying, hang on...");
    }, 4000);

    return () => {
      isMounted = false;
      clearTimeout(slowTimer);
    };
  }, [navigate]);

  return (
    <div className="min-h-screen flex items-center justify-center p-6">
      <div className="card max-w-md w-full p-8 text-center shadow-xs">
        <h2 className="font-display text-xl font-bold mb-2 text-[var(--ink)]">Please wait</h2>
        <p className="text-gray-500 text-sm">{message}</p>
      </div>
    </div>
  );
}

export default LoginRedirect;

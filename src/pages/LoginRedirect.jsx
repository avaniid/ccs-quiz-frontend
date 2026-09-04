import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import api from "../api/client";

function LoginRedirect() {
  const navigate = useNavigate();

  useEffect(() => {
    api
      .get("/verify")
      .then(() => {
        navigate("/instructions");
      })
      .catch(() => {
        navigate("/"); // not actually logged in, back to home
      });
  }, [navigate]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-900 text-white">
      <p>Verifying your login...</p>
    </div>
  );
}

export default LoginRedirect;
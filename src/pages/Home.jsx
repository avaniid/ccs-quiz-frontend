import { useNavigate } from "react-router-dom";

export default function Home() {
  const navigate = useNavigate();

  const handleLogin = () => {
    // TODO: replace with real Google OAuth redirect once backend confirms flow
    navigate("/instructions");
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
        <button onClick={handleLogin} className="btn-primary cursor-pointer">
          Sign in with Google
        </button>
      </div>
    </div>
  );
}
function Home() {
  const handleLogin = () => {
    window.location.href = `${import.meta.env.VITE_API_URL}/auth/google`;
  };

  return (
    <div className="min-h-screen flex flex-col items-center justify-center px-6">
      <div className="text-center mb-8">
        <img
          src="/ccs-logo.png"
          alt="CCS logo"
          className="w-20 h-20 mx-auto mb-4"
        />
        <p className="text-sm tracking-wide text-gray-500 mb-2">
          Creative Computing Society
        </p>
        <h1 className="font-display text-5xl font-bold text-[#0d1117]">
          Quiz <span className="text-[#2cbcb6]">Portal</span>
        </h1>
      </div>

      <div className="card p-8 max-w-sm w-full text-center">
        <p className="text-gray-600 mb-6 text-sm">
          Sign in with your college Google account to begin.
        </p>
        <button onClick={handleLogin} className="btn-primary w-full">
          Sign in with Google
        </button>
      </div>

      <a href="/instructions" className="text-xs text-gray-400 mt-6 underline">
        (dev only) skip login →
      </a>
    </div>
  );
}

export default Home;
import { useState } from "react";
import { useNavigate } from "react-router-dom";

export default function Instructions() {
  const [error, setError] = useState("");
  const navigate = useNavigate();

  const handleAccept = async () => {
    try {
      await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
      navigate("/quiz");
    } catch (err) {
      setError("Camera and microphone access is required to start the test.");
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-6">
      <div className="card max-w-3xl w-full p-8 md:p-10 shadow-xs">
        <h2 className="font-display text-2xl md:text-3xl font-bold mb-6 text-[var(--ink)]">
          Instructions
        </h2>
        <ul className="space-y-3 mb-8 text-gray-700 list-disc list-inside">
          <li>Do not switch tabs or exit fullscreen during the test.</li>
          <li>Camera and microphone must remain on.</li>
          <li>You will be auto-submitted after 5 warnings.</li>
        </ul>
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
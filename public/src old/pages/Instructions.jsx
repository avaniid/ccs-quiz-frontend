import { useNavigate } from "react-router-dom";

function Instructions() {
  const navigate = useNavigate();

  return (
    <div className="min-h-screen flex flex-col items-center justify-center gap-6 bg-gray-900 text-white px-6 text-center">
      <h1 className="text-2xl font-bold">Before you begin</h1>
      <ul className="text-left list-disc space-y-2 max-w-md">
        <li>Do not switch tabs or minimize the window once you start.</li>
        <li>Do not exit fullscreen during the quiz.</li>
        <li>Your webcam will be used to verify your identity.</li>
        <li>The quiz will auto-submit when the timer runs out.</li>
      </ul>
      <button
        onClick={() => navigate("/quiz")}
        className="bg-green-600 hover:bg-green-700 px-6 py-3 rounded-lg font-semibold"
      >
        Start Quiz
      </button>
    </div>
  );
}

export default Instructions;
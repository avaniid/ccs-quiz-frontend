import { BrowserRouter, Routes, Route } from "react-router-dom";
import Home from "./pages/Home";
import Instructions from "./pages/Instructions";
import Quiz from "./pages/Quiz";
import Submitted from "./pages/Submitted";
import Disqualified from "./pages/Disqualified";
import LoginRedirect from "./pages/LoginRedirect";
import { QuizProvider } from "./context/QuizContext";

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Home />} />
        {/* Where the backend's Google OAuth callback drops the candidate. */}
        <Route path="/login-redirect" element={<LoginRedirect />} />
        <Route path="/instructions" element={<Instructions />} />
        <Route path="/quiz" element={
          <QuizProvider>
            <Quiz />
          </QuizProvider>
        } />
        <Route path="/submitted" element={<Submitted />} />
        <Route path="/disqualified" element={<Disqualified />} />
      </Routes>
    </BrowserRouter>
  );
}

import { BrowserRouter, Routes, Route } from "react-router-dom";
import Home from "./pages/Home";
import LoginRedirect from "./pages/LoginRedirect";
import Instructions from "./pages/Instructions";
import Quiz from "./pages/Quiz";
import Submitted from "./pages/Submitted";
import Disqualified from "./pages/Disqualified";

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/login-redirect" element={<LoginRedirect />} />
        <Route path="/instructions" element={<Instructions />} />
        <Route path="/quiz" element={<Quiz />} />
        <Route path="/submitted" element={<Submitted />} />
        <Route path="/disqualified" element={<Disqualified />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
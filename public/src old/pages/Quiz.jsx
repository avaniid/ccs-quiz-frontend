import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import Webcam from "react-webcam";
import api from "../api/client";
import useAntiCheat from "../hooks/useAntiCheat";

// 🔧 TEMPORARY MOCK DATA — delete this whole block once the real backend is reachable
const MOCK_QUESTIONS = [
  {
    questionID: [1, 2, 3, 4, 5, 6, 7],
    question: "What does CSS stand for?",
    image: "",
    options: [
      { id: [10, 0, 0, 0, 0, 0, 1], value: "Cascading Style Sheets" },
      { id: [10, 0, 0, 0, 0, 0, 2], value: "Computer Style Sheets" },
      { id: [10, 0, 0, 0, 0, 0, 3], value: "Creative Style Sheets" },
    ],
  },
  {
    questionID: [1, 2, 3, 4, 5, 6, 8],
    question: "Which hook lets you run code after a component mounts?",
    image: "",
    options: [
      { id: [11, 0, 0, 0, 0, 0, 1], value: "useState" },
      { id: [11, 0, 0, 0, 0, 0, 2], value: "useEffect" },
      { id: [11, 0, 0, 0, 0, 0, 3], value: "useRef" },
    ],
  },
  {
    questionID: [1, 2, 3, 4, 5, 6, 9],
    question: "What HTTP method is typically used to submit a form?",
    image: "",
    options: [
      { id: [12, 0, 0, 0, 0, 0, 1], value: "GET" },
      { id: [12, 0, 0, 0, 0, 0, 2], value: "POST" },
      { id: [12, 0, 0, 0, 0, 0, 3], value: "DELETE" },
    ],
  },
];
// 🔧 END MOCK DATA

function Quiz() {
  const navigate = useNavigate();
  const flagsRaised = useAntiCheat();
  const webcamRef = useRef(null);
  const snapshotRef = useRef(null);

  const [questions, setQuestions] = useState([]);
  const [answers, setAnswers] = useState({});
  const [secondsLeft, setSecondsLeft] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function init() {
      // 🔧 MOCK MODE — comment this block back out once backend is live
      setQuestions(MOCK_QUESTIONS);
      setSecondsLeft(2 * 60); // 2 minutes, so you can actually watch it hit zero while testing
      setLoading(false);
      return;
      // 🔧 END MOCK MODE

      /* 🔧 REAL API CALLS — uncomment this whole block once backend is reachable, and delete the mock block above
      try {
        const submittedRes = await api.post("/quiz/submitted");
        if (submittedRes.status === 200) {
          navigate("/submitted");
          return;
        }
      } catch (err) {}

      try {
        const shiftsRes = await api.get("/quiz/shifts");
        setSecondsLeft(shiftsRes.data.test_duration * 60);

        const questionsRes = await api.get("/quiz/get");
        setQuestions(questionsRes.data.questions);
      } catch (err) {
        console.error("Failed to load quiz", err);
        navigate("/");
        return;
      }

      setLoading(false);
      */
    }
    init();
  }, [navigate]);

  useEffect(() => {
    if (!loading && webcamRef.current) {
      const t = setTimeout(() => {
        snapshotRef.current = webcamRef.current.getScreenshot();
      }, 1500);
      return () => clearTimeout(t);
    }
  }, [loading]);

  useEffect(() => {
    if (secondsLeft === null) return;
    if (secondsLeft <= 0) {
      handleSubmit();
      return;
    }
    const t = setTimeout(() => setSecondsLeft((s) => s - 1), 1000);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [secondsLeft]);

  useEffect(() => {
    const interval = setInterval(() => {
      if (flagsRaised.current >= 5) {
        clearInterval(interval);
        handleSubmit(true);
      }
    }, 1000);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function selectAnswer(questionIDArray, optionIDArray) {
    const key = JSON.stringify(questionIDArray);
    setAnswers((prev) => ({ ...prev, [key]: optionIDArray }));
  }

  async function handleSubmit(disqualified = false) {
    const responses = questions.map((q) => {
      const key = JSON.stringify(q.questionID);
      return {
        questionID: q.questionID,
        quizAnswers: answers[key] || [],
      };
    });

    const payload = {
      snapshot: snapshotRef.current || "",
      quiz_responses: responses,
      flagsRaised: flagsRaised.current,
    };

    // 🔧 MOCK MODE — just log it and navigate, don't actually call the backend yet
    console.log("Would submit:", payload);
    navigate(disqualified ? "/disqualified" : "/submitted");
    return;
    // 🔧 END MOCK MODE — delete the 3 lines above and uncomment below once backend is live

    /*
    try {
      await api.post("/quiz/submit", payload);
      navigate(disqualified ? "/disqualified" : "/submitted");
    } catch (err) {
      console.error("Submit failed", err);
    }
    */
  }

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-900 text-white">
        Loading quiz...
      </div>
    );
  }

  const minutes = String(Math.floor(secondsLeft / 60)).padStart(2, "0");
  const seconds = String(secondsLeft % 60).padStart(2, "0");

  return (
    <div className="min-h-screen bg-gray-900 text-white p-6">
      <Webcam
        ref={webcamRef}
        audio={false}
        screenshotFormat="image/jpeg"
        className="hidden"
      />

      <div className="flex justify-between items-center mb-6">
        <h1 className="text-xl font-bold">Quiz</h1>
        <span className="bg-red-600 px-4 py-2 rounded font-mono">
          {minutes}:{seconds}
        </span>
      </div>

      <div className="space-y-8 max-w-2xl mx-auto">
        {questions.map((q, idx) => {
          const key = JSON.stringify(q.questionID);
          return (
            <div key={key} className="bg-gray-800 p-4 rounded-lg">
              <p className="font-semibold mb-3">
                {idx + 1}. {q.question}
              </p>
              {q.image && (
                <img src={q.image} alt="" className="mb-3 rounded max-h-60" />
              )}
              <div className="space-y-2">
                {q.options.map((opt) => {
                  const optKey = JSON.stringify(opt.id);
                  const isSelected = JSON.stringify(answers[key]) === optKey;
                  return (
                    <button
                      key={optKey}
                      onClick={() => selectAnswer(q.questionID, opt.id)}
                      className={`block w-full text-left px-4 py-2 rounded border ${
                        isSelected
                          ? "bg-blue-600 border-blue-400"
                          : "bg-gray-700 border-gray-600 hover:bg-gray-600"
                      }`}
                    >
                      {opt.value}
                    </button>
                  );
                })}
              </div>
            </div>
          );
        })}
      </div>

      <div className="max-w-2xl mx-auto mt-8">
        <button
          onClick={() => handleSubmit(false)}
          className="w-full bg-green-600 hover:bg-green-700 py-3 rounded-lg font-semibold"
        >
          Submit Quiz
        </button>
      </div>
    </div>
  );
}

export default Quiz;
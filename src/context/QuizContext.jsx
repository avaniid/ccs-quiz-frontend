import { createContext, useContext, useState, useEffect, useCallback, useRef } from "react";
import {
  fetchQuestions,
  fetchQuizConfig,
  checkAlreadySubmitted,
  readApiError,
} from "../services/api";

// Used until /quiz/shifts reports the real length configured on the backend.
export const EXAM_DURATION_SECONDS = 1800;

const QuizContext = createContext(null);

export function QuizProvider({ children }) {
  const [questions, setQuestions] = useState([]);
  const [answers, setAnswers] = useState({});
  const [currentIndex, setCurrentIndex] = useState(0);
  const [markedForReview, setMarkedForReview] = useState(new Set());
  const [visited, setVisited] = useState(new Set());
  const [warningCount, setWarningCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [unauthorized, setUnauthorized] = useState(false);
  const [timeRemaining, setTimeRemaining] = useState(EXAM_DURATION_SECONDS);
  const [submitted, setSubmitted] = useState(false);

  const warningCountRef = useRef(0);

  // Load the exam length, the already-submitted flag and the questions on mount.
  useEffect(() => {
    let isMounted = true;
    async function load() {
      try {
        // The duration is public config; a failure here just leaves the default.
        try {
          const { durationSeconds } = await fetchQuizConfig();
          if (isMounted && durationSeconds) {
            setTimeRemaining(durationSeconds);
          }
        } catch (configError) {
          console.warn("Could not load quiz config, using default duration:", configError);
        }

        // A second attempt must not hand out the paper again.
        if (await checkAlreadySubmitted()) {
          if (isMounted) setSubmitted(true);
          return;
        }

        const data = await fetchQuestions();
        if (isMounted) setQuestions(data);
      } catch (error) {
        console.error("Failed to load questions:", error);
        if (isMounted) {
          setUnauthorized(error?.response?.status === 401);
          setLoadError(readApiError(error, "Could not load your quiz."));
        }
      } finally {
        if (isMounted) setLoading(false);
      }
    }
    load();
    return () => {
      isMounted = false;
    };
  }, []);

  // Mark current question as visited whenever currentIndex or questions change
  useEffect(() => {
    if (questions.length > 0 && questions[currentIndex]) {
      const qId = questions[currentIndex].id;
      setVisited((prev) => {
        if (prev.has(qId)) return prev;
        const next = new Set(prev);
        next.add(qId);
        return next;
      });
    }
  }, [currentIndex, questions]);

  const answerQuestion = useCallback((questionId, optionId) => {
    setAnswers((prev) => ({ ...prev, [questionId]: optionId }));
  }, []);

  const clearResponse = useCallback((questionId) => {
    setAnswers((prev) => {
      const copy = { ...prev };
      delete copy[questionId];
      return copy;
    });
  }, []);

  const toggleMarkForReview = useCallback((questionId) => {
    setMarkedForReview((prev) => {
      const next = new Set(prev);
      if (next.has(questionId)) {
        next.delete(questionId);
      } else {
        next.add(questionId);
      }
      return next;
    });
  }, []);

  const goToQuestion = useCallback((index) => {
    setCurrentIndex(index);
  }, []);

  const nextQuestion = useCallback(() => {
    setCurrentIndex((prev) => {
      if (prev < questions.length - 1) {
        return prev + 1;
      }
      return prev;
    });
  }, [questions.length]);

  const incrementWarning = useCallback(() => {
    warningCountRef.current += 1;
    setWarningCount(warningCountRef.current);
    return warningCountRef.current;
  }, []);

  const markSubmitted = useCallback(() => {
    setSubmitted(true);
  }, []);

  const getQuestionStatus = useCallback((questionId) => {
    if (markedForReview.has(questionId)) {
      return "marked";
    }
    if (answers[questionId] !== undefined) {
      return "answered";
    }
    if (visited.has(questionId)) {
      return "not-answered";
    }
    return "not-visited";
  }, [markedForReview, answers, visited]);

  const value = {
    questions,
    answers,
    currentIndex,
    markedForReview,
    visited,
    warningCount,
    loading,
    loadError,
    unauthorized,
    timeRemaining,
    setTimeRemaining,
    submitted,
    markSubmitted,
    answerQuestion,
    clearResponse,
    toggleMarkForReview,
    goToQuestion,
    nextQuestion,
    incrementWarning,
    getQuestionStatus,
  };

  return <QuizContext.Provider value={value}>{children}</QuizContext.Provider>;
}

export function useQuiz() {
  const context = useContext(QuizContext);
  if (!context) {
    throw new Error("useQuiz must be used within a QuizProvider");
  }
  return context;
}

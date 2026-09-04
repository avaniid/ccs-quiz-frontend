import { createContext, useContext, useState, useEffect, useCallback, useRef } from "react";
import { fetchQuestions } from "../services/api";

const QuizContext = createContext(null);

export function QuizProvider({ children }) {
  const [questions, setQuestions] = useState([]);
  const [answers, setAnswers] = useState({});
  const [currentIndex, setCurrentIndex] = useState(0);
  const [markedForReview, setMarkedForReview] = useState(new Set());
  const [visited, setVisited] = useState(new Set());
  const [warningCount, setWarningCount] = useState(0);
  const [loading, setLoading] = useState(true);

  const warningCountRef = useRef(0);

  // Load questions on mount
  useEffect(() => {
    let isMounted = true;
    async function load() {
      try {
        const data = await fetchQuestions();
        if (isMounted) {
          setQuestions(data || []);
        }
      } catch (error) {
        console.error("Failed to load questions:", error);
      } finally {
        if (isMounted) {
          setLoading(false);
        }
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

  const answerQuestion = useCallback((questionId, option) => {
    setAnswers((prev) => ({ ...prev, [questionId]: option }));
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


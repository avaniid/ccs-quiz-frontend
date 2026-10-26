import { createContext, useContext, useState, useEffect, useCallback, useRef } from "react";
import {
  fetchQuestions,
  fetchQuizConfig,
  checkAlreadySubmitted,
  readApiError,
} from "../services/api";
import { paperKeyOf, readProgress, saveProgress } from "../services/quizProgress";

// Used only against an older backend that does not send a deadline, and only
// until /quiz/shifts reports the real length.
export const EXAM_DURATION_SECONDS = 1800;

const QuizContext = createContext(null);

export function QuizProvider({ children }) {
  const [questions, setQuestions] = useState([]);
  const [answers, setAnswers] = useState({});
  const [currentIndex, setCurrentIndex] = useState(0);
  const [markedForReview, setMarkedForReview] = useState(new Set());
  const [visited, setVisited] = useState(new Set());
  const [warningCount, setWarningCount] = useState(0);
  const [strikeCount, setStrikeCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [unauthorized, setUnauthorized] = useState(false);
  const [deadlineMs, setDeadlineMs] = useState(null);
  const [submitted, setSubmitted] = useState(false);

  const warningCountRef = useRef(0);
  // Violations that count toward the auto-submit limit (see Quiz.jsx).
  const strikeCountRef = useRef(0);
  const paperKeyRef = useRef("");

  // Load the already-submitted flag, then the paper and its deadline.
  useEffect(() => {
    let isMounted = true;
    async function load() {
      try {
        // A second attempt must not hand out the paper again.
        if (await checkAlreadySubmitted()) {
          if (isMounted) setSubmitted(true);
          return;
        }

        const paper = await fetchQuestions();
        let deadline = paper.deadlineMs;
        if (deadline == null) {
          let seconds = EXAM_DURATION_SECONDS;
          try {
            const { durationSeconds } = await fetchQuizConfig();
            if (durationSeconds) seconds = durationSeconds;
          } catch (configError) {
            console.warn("Could not load quiz config, using default duration:", configError);
          }
          deadline = Date.now() + seconds * 1000;
        }
        if (!isMounted) return;

        const key = paperKeyOf(paper.questions);
        paperKeyRef.current = key;
        const saved = readProgress(key);
        if (saved) {
          // Only restore choices that still point at a real option.
          const validAnswers = {};
          for (const q of paper.questions) {
            const chosen = saved.answers?.[q.id];
            if (chosen !== undefined && q.options.some((o) => o.id === chosen)) {
              validAnswers[q.id] = chosen;
            }
          }
          setAnswers(validAnswers);
          setMarkedForReview(new Set(saved.marked || []));
          setVisited(new Set(saved.visited || []));
          const idx = Number(saved.currentIndex) || 0;
          setCurrentIndex(idx < paper.questions.length ? idx : 0);
        }
        // The server counts violations too; take whichever is higher so
        // clearing storage or editing it cannot wipe the slate.
        warningCountRef.current = Math.max(paper.flags, Number(saved?.warnings) || 0);
        strikeCountRef.current = Math.max(paper.strikes, Number(saved?.strikes) || 0);
        setWarningCount(warningCountRef.current);
        setStrikeCount(strikeCountRef.current);

        setDeadlineMs(deadline);
        setQuestions(paper.questions);
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

  // Mirror progress to storage while the attempt is live.
  useEffect(() => {
    if (!questions.length || submitted) return;
    saveProgress({
      paperKey: paperKeyRef.current,
      answers,
      marked: [...markedForReview],
      visited: [...visited],
      currentIndex,
      warnings: warningCountRef.current,
      strikes: strikeCountRef.current,
    });
  }, [questions, answers, markedForReview, visited, currentIndex, warningCount, submitted]);

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

  /** Records a violation; returns the running warning count (all types). */
  const incrementWarning = useCallback((isStrike) => {
    warningCountRef.current += 1;
    if (isStrike) strikeCountRef.current += 1;
    setWarningCount(warningCountRef.current);
    setStrikeCount(strikeCountRef.current);
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
    strikeCount,
    loading,
    loadError,
    unauthorized,
    deadlineMs,
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

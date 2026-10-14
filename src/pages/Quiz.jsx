import { useState, useCallback, useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { useQuiz } from "../context/QuizContext";
import { clearQuizProgress } from "../services/quizProgress";
import {
  submitQuiz as apiSubmitQuiz,
  getGoogleLoginUrl,
  reportViolation,
  isAlreadySubmittedError,
  readApiError,
} from "../services/api";
import useTimer from "../hooks/useTimer";
import useTabSwitchGuard from "../hooks/useTabSwitchGuard";
import useFullscreenGuard from "../hooks/useFullscreenGuard";
import useCameraStream from "../hooks/useCameraStream";
import useFaceGuard from "../hooks/useFaceGuard";
import useObjectGuard from "../hooks/useObjectGuard";
import useVoiceGuard, { ENROLL_SECONDS } from "../hooks/useVoiceGuard";
import { releaseCameraStream } from "../hooks/cameraStore";
import WarningModal from "../components/WarningModal";
import VoiceEnrollmentOverlay from "../components/VoiceEnrollmentOverlay";

const MAX_WARNINGS = 15;

// Voice matching is the least reliable guard (a fan, a cough or the
// candidate's own changing tone can trip it), so it still warns and is
// reported, but does not count toward the auto-submit limit.
const NON_STRIKE_VIOLATIONS = new Set(["voice-mismatch"]);

// Backoff between submit attempts. Covers a cold-starting backend (Render's
// free tier takes up to a minute to wake) and short network drops.
const SUBMIT_RETRY_DELAYS_MS = [0, 1500, 3000, 6000, 12000, 20000];

export default function Quiz() {
  const navigate = useNavigate();
  const {
    questions,
    answers,
    currentIndex,
    markedForReview,
    loading,
    loadError,
    unauthorized,
    warningCount,
    strikeCount,
    deadlineMs,
    submitted,
    answerQuestion,
    clearResponse,
    toggleMarkForReview,
    goToQuestion,
    nextQuestion,
    incrementWarning,
    getQuestionStatus,
  } = useQuiz();

  const [modalVisible, setModalVisible] = useState(false);
  const [violationType, setViolationType] = useState("");
  const isSubmittingRef = useRef(false);
  const snapshotCanvasRef = useRef(null);
  // idle | submitting | failed
  const [submitPhase, setSubmitPhase] = useState("idle");
  const [submitError, setSubmitError] = useState("");
  const [submitUnauthorized, setSubmitUnauthorized] = useState(false);
  const submitReasonRef = useRef("manual");
  const snapshotRef = useRef(null);

  // The paper is on screen. Everything that can end the attempt (timer,
  // warnings, fullscreen enforcement) waits for this, so a "quiz not started"
  // or "no paper" screen can never auto-submit an empty attempt.
  const active = !loading && !loadError && questions.length > 0 && !submitted;
  const activeRef = useRef(active);
  useEffect(() => {
    activeRef.current = active;
  }, [active]);
  // Holds the latest submitQuiz so handleViolation (created before submitQuiz,
  // since the camera/voice hooks need handleViolation up front) never calls a
  // stale closure.
  const submitQuizRef = useRef(() => {});

  // Redirect to /submitted if already submitted
  useEffect(() => {
    if (submitted) {
      navigate("/submitted", { replace: true });
    }
  }, [submitted, navigate]);


  // Anti-cheat violation handler (shared by every guard: tab/fullscreen/camera-based)
  const handleViolation = useCallback(
    (type) => {
      if (!activeRef.current || isSubmittingRef.current) return;

      const isStrike = !NON_STRIKE_VIOLATIONS.has(type);
      reportViolation(type, isStrike);
      const strikes = incrementWarning(isStrike);
      setViolationType(type);
      setModalVisible(true);

      if (strikes >= MAX_WARNINGS) {
        submitQuizRef.current("warnings");
      }
    },
    [incrementWarning]
  );

  // Camera + mic stream, shared across all camera/voice proctoring hooks
  const { stream: cameraStream, videoRef, error: cameraError } = useCameraStream(handleViolation);
  useFaceGuard(videoRef, cameraStream, handleViolation);
  useObjectGuard(videoRef, cameraStream, handleViolation);
  const { phase: voicePhase, secondsLeft: enrollSecondsLeft, startEnrollment } =
    useVoiceGuard(cameraStream, handleViolation);

  // Start voice enrollment once the mic is ready AND the paper is on screen.
  // Starting on the stream alone ran most of the 10 seconds behind the
  // loading screen, where the "please speak" overlay is not shown.
  useEffect(() => {
    if (cameraStream && active) {
      startEnrollment();
    }
  }, [cameraStream, active, startEnrollment]);

  const captureSnapshot = useCallback(() => {
    const video = videoRef.current;
    if (!video || video.videoWidth === 0) return null;
    const canvas = snapshotCanvasRef.current;
    canvas.width = video.videoWidth;
    canvas.height = video.videoHeight;
    canvas.getContext("2d").drawImage(video, 0, 0);
    try {
      return canvas.toDataURL("image/jpeg", 0.8);
    } catch {
      return null;
    }
  }, [videoRef]);

  // Submit with retries. The attempt only counts as over once the server has
  // it: this used to flag the attempt submitted first and move on even when
  // the request failed, so a sleeping backend or a network blip silently threw
  // the answers away while the candidate was shown "submitted".
  const submitQuiz = useCallback(
    async (reason = "manual") => {
      if (isSubmittingRef.current || submitted) return;
      isSubmittingRef.current = true;
      submitReasonRef.current = reason;

      // Grab the frame while the <video> is still mounted; the submitting
      // screen below replaces it. Kept for retries.
      if (snapshotRef.current === null) snapshotRef.current = captureSnapshot() || "";

      setSubmitPhase("submitting");
      setSubmitError("");
      setSubmitUnauthorized(false);

      let lastError = null;
      for (const delay of SUBMIT_RETRY_DELAYS_MS) {
        if (delay) await new Promise((resolve) => setTimeout(resolve, delay));
        try {
          await apiSubmitQuiz(questions, answers, warningCount, snapshotRef.current);
          lastError = null;
          break;
        } catch (err) {
          // An earlier try that timed out on our side may have landed.
          if (isAlreadySubmittedError(err)) {
            lastError = null;
            break;
          }
          lastError = err;
          const status = err?.response?.status;
          // A 4xx (time over, bad session) will not fix itself by retrying.
          if (status >= 400 && status < 500 && status !== 408 && status !== 429) break;
        }
      }

      if (lastError) {
        console.error("Failed to submit quiz:", lastError);
        setSubmitUnauthorized(lastError?.response?.status === 401);
        setSubmitError(readApiError(lastError, "Your answers could not be submitted."));
        setSubmitPhase("failed");
        // isSubmittingRef stays set: no more warnings or auto-submits while
        // the failure screen is up. Retry resets it.
        return;
      }

      clearQuizProgress();
      // Attempt is over: turn the camera and mic off so the indicator light
      // does not stay on after the candidate leaves the quiz.
      releaseCameraStream();
      navigate("/submitted", { replace: true, state: { reason } });
    },
    [questions, answers, warningCount, submitted, navigate, captureSnapshot]
  );

  const retrySubmit = useCallback(() => {
    isSubmittingRef.current = false;
    submitQuiz(submitReasonRef.current);
  }, [submitQuiz]);

  useEffect(() => {
    submitQuizRef.current = submitQuiz;
  }, [submitQuiz]);

  // Counts down to the server's deadline, so a reload does not reset it.
  const timeRemaining = useTimer(deadlineMs, active, () => {
    submitQuizRef.current("timeout");
  });

  useTabSwitchGuard(handleViolation, active);
  useFullscreenGuard(handleViolation, active);

  if (submitPhase !== "idle") {
    const failed = submitPhase === "failed";
    return (
      <div className="min-h-screen flex items-center justify-center p-6">
        <div className="card max-w-md w-full p-8 text-center shadow-xs">
          <h2
            className={`font-display text-xl font-bold mb-2 ${
              failed ? "text-red-600" : "text-[var(--ink)]"
            }`}
          >
            {failed ? "Submission failed" : "Submitting your answers..."}
          </h2>
          <p className="text-gray-500 text-sm mb-6">
            {failed
              ? `${submitError} Your answers are saved on this device - do not close this tab.`
              : "Please keep this tab open. This can take up to a minute."}
          </p>
          {failed &&
            (submitUnauthorized ? (
              <button
                onClick={() => {
                  // Answers and the deadline survive the sign-in round trip.
                  window.location.href = getGoogleLoginUrl();
                }}
                className="btn-primary cursor-pointer"
              >
                Sign in again
              </button>
            ) : (
              <button onClick={retrySubmit} className="btn-primary cursor-pointer">
                Retry submit
              </button>
            ))}
        </div>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center p-6">
        <div className="card max-w-md w-full p-8 text-center shadow-xs">
          <h2 className="font-display text-xl font-bold mb-2 text-[var(--ink)]">
            Loading Quiz...
          </h2>
          <p className="text-gray-500 text-sm">Please wait while questions are loaded.</p>
        </div>
      </div>
    );
  }

  // Covers an expired session, a shift that hasn't opened yet, and a candidate
  // with no paper assigned — the backend reports all three on /quiz/get.
  if (loadError || questions.length === 0) {
    return (
      <div className="min-h-screen flex items-center justify-center p-6">
        <div className="card max-w-md w-full p-8 text-center shadow-xs">
          <h2 className="font-display text-xl font-bold mb-2 text-[var(--ink)]">
            Quiz Unavailable
          </h2>
          <p className="text-gray-500 text-sm mb-6">
            {loadError || "No questions have been assigned to you yet."}
          </p>
          {unauthorized ? (
            <button
              onClick={() => {
                window.location.href = getGoogleLoginUrl();
              }}
              className="btn-primary cursor-pointer"
            >
              Sign in again
            </button>
          ) : (
            <button
              onClick={() => window.location.reload()}
              className="btn-primary cursor-pointer"
            >
              Retry
            </button>
          )}
        </div>
      </div>
    );
  }

  if (cameraError) {
    return (
      <div className="min-h-screen flex items-center justify-center p-6">
        <div className="card max-w-md w-full p-8 text-center shadow-xs">
          <h2 className="font-display text-xl font-bold mb-2 text-red-600">
            Camera/Microphone Required
          </h2>
          <p className="text-gray-500 text-sm mb-2">
            {cameraError?.name === "NotAllowedError"
              ? "Camera or microphone access was blocked. Click the camera icon in your browser's address bar, allow both, then reload."
              : cameraError?.name === "NotReadableError"
                ? "Your camera or microphone is being used by another app. Close it (Zoom, Meet, Teams, another tab) and reload."
                : cameraError?.name === "NotFoundError"
                  ? "No camera or microphone was found. Connect one and reload."
                  : "Proctoring could not access your camera and microphone. Please allow access and reload this page to continue."}
          </p>
          {/* The error name is what makes a report like "camera nahi chal raha"
              actionable, so keep it visible rather than only in the console. */}
          <p className="text-xs text-gray-400 mb-6">
            {cameraError?.name || "Error"}
            {cameraError?.message ? `: ${cameraError.message}` : ""}
          </p>
          <button
            onClick={() => window.location.reload()}
            className="btn-primary cursor-pointer"
          >
            Reload
          </button>
        </div>
      </div>
    );
  }

  if (!cameraStream) {
    return (
      <div className="min-h-screen flex items-center justify-center p-6">
        <div className="card max-w-md w-full p-8 text-center shadow-xs">
          <h2 className="font-display text-xl font-bold mb-2 text-[var(--ink)]">
            Setting Up Proctoring...
          </h2>
          <p className="text-gray-500 text-sm">Requesting camera and microphone access.</p>
        </div>
      </div>
    );
  }

  const currentQuestion = questions[currentIndex] || questions[0];
  const currentAnswer = answers[currentQuestion.id];

  const formatTime = (totalSeconds) => {
    const m = Math.floor(totalSeconds / 60);
    const s = totalSeconds % 60;
    return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  };

  const getPaletteButtonClass = (status, isCurrent) => {
    let base = "";
     switch (status) {
      case "answered":
        base = "bg-green-600 text-white";
        break;
      case "marked":
        base = "bg-purple-600 text-white";
        break;
      case "not-answered":
        base = "bg-red-600 text-white";
        break;
      case "not-visited":
      default:
        base = "bg-gray-100 border border-[var(--border)] text-gray-500";
        break;
    }
    return `${base} ${isCurrent ? "ring-2 ring-[var(--blue)]" : ""}`;
  };

  return (
    <div className="min-h-screen p-6 md:p-8">
      <div className="max-w-7xl mx-auto">
        {/* Header with Title and Prominent Timer */}
        <div className="flex flex-wrap items-center justify-between gap-4 mb-6">
          <div>
            <h1 className="font-display text-2xl md:text-3xl font-bold text-[var(--ink)]">
              CCS Recruitment Quiz
            </h1>
          </div>

          {/* Prominent Timer */}
          <div className="flex items-center gap-2 bg-white px-5 py-2.5 rounded-lg border border-[var(--border)] shadow-2xs">
            <span className="text-xs uppercase font-bold text-gray-500 tracking-wider">
              Time Left:
            </span>
            <span
              className={`font-display text-2xl font-bold ${
                timeRemaining <= 300 ? "text-red-600 animate-pulse" : "text-[var(--blue)]"
              }`}
            >
              {formatTime(timeRemaining)}
            </span>
          </div>

          <button
            onClick={() => submitQuiz("manual")}
            className="btn-primary cursor-pointer text-sm"
          >
            Submit Test
          </button>
        </div>

        {/* Content Layout */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Main Question Display */}
          <div className="lg:col-span-2">
            <div className="card p-6 md:p-8 shadow-xs">
              {/* Question Meta */}
              <div className="flex items-center justify-between pb-4 mb-6 border-b border-[var(--border)]">
                <span className="font-semibold text-sm text-gray-500 uppercase tracking-wider">
                  Question {currentIndex + 1} of {questions.length}
                </span>
                {markedForReview.has(currentQuestion.id) && (
                  <span className="text-xs font-semibold px-2.5 py-1 rounded bg-[var(--cyan)] text-white">
                    Marked for Review
                  </span>
                )}
              </div>

              {/* Question Text */}
              <p className="font-display text-lg md:text-xl font-semibold mb-6 text-[var(--ink)]">
                {currentQuestion.text}
              </p>

              {/* Optional per-question image (cloudinary link from the backend) */}
              {currentQuestion.image && (
                <img
                  src={currentQuestion.image}
                  alt=""
                  className="mb-6 max-h-72 rounded-lg border border-[var(--border)]"
                />
              )}

              {/* Options */}
              <div className="space-y-3 mb-8">
                {(currentQuestion.options || []).map((opt) => {
                  const isSelected = currentAnswer === opt.id;

                  return (
                    <button
                      key={opt.id}
                      type="button"
                      onClick={() => answerQuestion(currentQuestion.id, opt.id)}
                      className={`w-full text-left p-4 rounded-lg border transition-colors cursor-pointer flex items-center gap-3 ${
                        isSelected
                          ? "border-[var(--blue)] bg-blue-50/50 text-[var(--ink)] font-medium"
                          : "border-[var(--border)] bg-white hover:bg-[var(--bg)] text-[var(--ink)]"
                      }`}
                    >
                      <span
                        className={`w-5 h-5 rounded-full border flex items-center justify-center text-xs shrink-0 ${
                          isSelected
                            ? "border-[var(--blue)] bg-[var(--blue)] text-white"
                            : "border-gray-400 bg-white"
                        }`}
                      >
                        {isSelected && (
                          <span className="w-2 h-2 rounded-full bg-white" />
                        )}
                      </span>
                      <span>{opt.value}</span>
                    </button>
                  );
                })}
              </div>

              {/* Action Buttons */}
              <div className="flex flex-wrap items-center justify-between gap-3 pt-6 border-t border-[var(--border)]">
                <div className="flex flex-wrap items-center gap-3">
                  <button
                    type="button"
                    onClick={() => clearResponse(currentQuestion.id)}
                    className="bg-white border border-[var(--border)] text-[var(--ink)] font-semibold rounded-[8px] px-7 py-3 hover:bg-[var(--bg)] transition-colors cursor-pointer"
                  >
                    Clear Response
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      toggleMarkForReview(currentQuestion.id);
                      nextQuestion();
                    }}
                    className="bg-white border border-[var(--border)] text-[var(--ink)] font-semibold rounded-[8px] px-7 py-3 hover:bg-[var(--bg)] transition-colors cursor-pointer"
                  >
                    Mark for Review & Next
                  </button>
                </div>
                <button
                  type="button"
                  onClick={nextQuestion}
                  className="btn-primary cursor-pointer"
                >
                  Save & Next
                </button>
              </div>
            </div>
          </div>

          {/* Palette Sidebar */}
          <div className="space-y-6">
            {/* Proctoring camera widget */}
            <div className="card p-4 shadow-xs">
              <div className="flex items-center gap-2 mb-3">
                <span className="w-2.5 h-2.5 rounded-full bg-red-500 animate-pulse shrink-0" />
                <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
                  Proctoring Active
                </span>
              </div>
              <div className="relative rounded-lg overflow-hidden border border-[var(--border)] bg-black aspect-4/3">
                <video
                  ref={videoRef}
                  autoPlay
                  playsInline
                  muted
                  className="w-full h-full object-cover [transform:scaleX(-1)]"
                />
              </div>
            </div>

            <div className="card p-6 shadow-xs">
              <h2 className="font-display text-lg font-bold text-[var(--ink)] mb-4">
                Questions
              </h2>

              {/* Legend */}
              <div className="grid grid-cols-2 gap-2 text-xs mb-6 p-3 rounded-lg bg-[var(--bg)] border border-[var(--border)]">
                                <div className="flex items-center gap-2">
                  <span className="w-3.5 h-3.5 rounded bg-green-600 inline-block shrink-0" />
                  <span className="text-gray-600">Answered</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="w-3.5 h-3.5 rounded bg-purple-600 inline-block shrink-0" />
                  <span className="text-gray-600">Marked</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="w-3.5 h-3.5 rounded bg-red-600 inline-block shrink-0" />
                  <span className="text-gray-600">Not Answered</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="w-3.5 h-3.5 rounded bg-gray-100 border border-[var(--border)] inline-block shrink-0" />
                  <span className="text-gray-600">Not Visited</span>
                </div>
              </div>

              {/* Grid of Palette buttons */}
              <div className="grid grid-cols-5 gap-2.5">
                {questions.map((q, idx) => {
                  const status = getQuestionStatus(q.id);
                  const isCurrent = currentIndex === idx;
                  return (
                    <button
                      key={q.id || idx}
                      type="button"
                      onClick={() => goToQuestion(idx)}
                      className={`w-10 h-10 rounded-lg flex items-center justify-center font-semibold text-sm cursor-pointer transition-all ${getPaletteButtonClass(
                        status,
                        isCurrent
                      )}`}
                    >
                      {idx + 1}
                    </button>
                  );
                })}
              </div>

              {/* Sidebar Submit Button */}
              <div className="mt-8 pt-6 border-t border-[var(--border)]">
                <button
                  type="button"
                  onClick={() => submitQuiz("manual")}
                  className="btn-primary w-full cursor-pointer"
                >
                  Submit Test
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Hidden canvas used only to capture a submit-time snapshot */}
      <canvas ref={snapshotCanvasRef} className="hidden" />

      {/* Voice enrollment overlay (blocks interaction until enrolled) */}
      <VoiceEnrollmentOverlay
        visible={voicePhase === "enrolling"}
        secondsLeft={enrollSecondsLeft}
        totalSeconds={ENROLL_SECONDS}
      />

      {/* Warning Modal */}
      <WarningModal
        visible={modalVisible}
        warningCount={strikeCount}
        maxWarnings={MAX_WARNINGS}
        violationType={violationType}
        onDismiss={() => {
          setModalVisible(false);
          // Re-entering fullscreen needs a real user gesture (this click) —
          // the guard's own automatic retry silently fails without one.
          if (violationType === "fullscreen-exit" && !document.fullscreenElement) {
            document.documentElement.requestFullscreen?.().catch(() => {});
          }
        }}
      />
    </div>
  );
}

import { useNavigate } from "react-router-dom";
import { useQuiz } from "../context/QuizContext";

export default function Quiz() {
  const navigate = useNavigate();
  const {
    questions,
    answers,
    currentIndex,
    markedForReview,
    loading,
    answerQuestion,
    clearResponse,
    toggleMarkForReview,
    goToQuestion,
    nextQuestion,
    getQuestionStatus,
  } = useQuiz();

  const handleSubmit = () => {
    navigate("/submitted");
  };

  if (loading || questions.length === 0) {
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

  const currentQuestion = questions[currentIndex] || questions[0];
  const currentAnswer = answers[currentQuestion.id];

  const getPaletteButtonClass = (status, isCurrent) => {
    let base = "";
    switch (status) {
      case "answered":
        base = "bg-[var(--blue)] text-white";
        break;
      case "marked":
        base = "bg-[var(--cyan)] text-white";
        break;
      case "not-answered":
        base = "bg-red-100 border border-red-400 text-red-700";
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
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <h1 className="font-display text-2xl md:text-3xl font-bold text-[var(--ink)]">
            Quiz Assessment
          </h1>
          <button onClick={handleSubmit} className="btn-primary cursor-pointer text-sm">
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
                {currentQuestion.text || currentQuestion.question}
              </p>

              {/* Options */}
              <div className="space-y-3 mb-8">
                {(currentQuestion.options || []).map((opt, idx) => {
                  const optValue = typeof opt === "object" ? opt.value : opt;
                  const isSelected = currentAnswer === optValue;

                  return (
                    <button
                      key={idx}
                      type="button"
                      onClick={() => answerQuestion(currentQuestion.id, optValue)}
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
                      <span>{optValue}</span>
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
            <div className="card p-6 shadow-xs">
              <h2 className="font-display text-lg font-bold text-[var(--ink)] mb-4">
                Question Palette
              </h2>

              {/* Legend */}
              <div className="grid grid-cols-2 gap-2 text-xs mb-6 p-3 rounded-lg bg-[var(--bg)] border border-[var(--border)]">
                <div className="flex items-center gap-2">
                  <span className="w-3.5 h-3.5 rounded bg-[var(--blue)] inline-block shrink-0" />
                  <span className="text-gray-600">Answered</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="w-3.5 h-3.5 rounded bg-[var(--cyan)] inline-block shrink-0" />
                  <span className="text-gray-600">Marked</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="w-3.5 h-3.5 rounded bg-red-100 border border-red-400 inline-block shrink-0" />
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
                  onClick={handleSubmit}
                  className="btn-primary w-full cursor-pointer"
                >
                  Submit Test
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
// Progress is mirrored to localStorage so a refresh or a browser crash does not
// wipe the candidate's answers. It is keyed by the paper, so a different paper
// (another account on the same machine, a reassigned shift) never inherits it.
const PROGRESS_KEY = "ccs_quiz_progress";

export function paperKeyOf(questions) {
  return questions
    .map((q) => q.id)
    .sort()
    .join("|");
}

export function readProgress(paperKey) {
  try {
    const saved = JSON.parse(localStorage.getItem(PROGRESS_KEY) || "null");
    return saved?.paperKey === paperKey ? saved : null;
  } catch {
    return null;
  }
}

export function clearQuizProgress() {
  try {
    localStorage.removeItem(PROGRESS_KEY);
  } catch {
    // Storage blocked: nothing was saved either.
  }
}

export function saveProgress(progress) {
  try {
    localStorage.setItem(PROGRESS_KEY, JSON.stringify(progress));
  } catch {
    // Private mode / full storage: the attempt still works, just without
    // crash recovery.
  }
}

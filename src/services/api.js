import api, { API_BASE_URL } from "../api/client";

/**
 * The Go models carry only bson tags, so Gin serialises the Go field names
 * verbatim (`QuestionID`, `Options`, `Value`, ...) and the `[7]byte` ids arrive
 * as arrays of seven numbers. We keep those raw arrays around untouched — the
 * submit endpoint validates answers by comparing them against the ids it handed
 * out — and derive a plain string alongside them to use as a React key and as
 * the key of the answers map. Lowercase/bson spellings are accepted too so the
 * frontend keeps working if json tags are added to the models later.
 */
function toKey(rawId, fallback) {
  if (Array.isArray(rawId)) return rawId.join("-");
  if (rawId !== undefined && rawId !== null && rawId !== "") return String(rawId);
  return fallback;
}

function pick(obj, ...names) {
  for (const name of names) {
    if (obj?.[name] !== undefined && obj[name] !== null) return obj[name];
  }
  return undefined;
}

function normalizeOption(raw, questionIndex, optionIndex) {
  if (typeof raw === "string") {
    return { id: `q${questionIndex}-o${optionIndex}`, rawId: null, value: raw };
  }
  const rawId = pick(raw, "Id", "id", "option_id");
  return {
    id: toKey(rawId, `q${questionIndex}-o${optionIndex}`),
    rawId: rawId ?? null,
    value: pick(raw, "Value", "value", "option_value") ?? "",
  };
}

function normalizeQuestion(raw, index) {
  const rawId = pick(raw, "QuestionID", "questionID", "questionId", "questionid");
  return {
    id: toKey(rawId, `q${index}`),
    rawId: rawId ?? null,
    text: pick(raw, "Question", "question", "quizQuestions") ?? "",
    image: pick(raw, "Image", "image", "image_url") ?? "",
    shift: pick(raw, "Shift", "shift") ?? null,
    options: (pick(raw, "Options", "options") ?? []).map((opt, optionIndex) =>
      normalizeOption(opt, index, optionIndex)
    ),
  };
}

/** Turns an axios failure into a message worth showing the candidate. */
export function readApiError(err, fallback = "Something went wrong. Please try again.") {
  return (
    err?.response?.data?.message ||
    err?.response?.data?.error ||
    (err?.response ? fallback : "Could not reach the server. Check your connection.")
  );
}

/** Full URL of the backend's Google OAuth entry point (a top-level redirect). */
export function getGoogleLoginUrl() {
  return `${API_BASE_URL}/auth/google`;
}

/** Resolves with the signed-in user, rejects (401) when the session is invalid. */
export async function verifySession() {
  const { data } = await api.get("/verify");
  return data;
}

export async function logout() {
  const { data } = await api.get("/logout");
  return data;
}

/**
 * Shift timings plus the exam length. The backend's TEST_DURATION is expressed
 * in minutes (default 80), so convert it to the seconds the timer counts down.
 */
export async function fetchQuizConfig() {
  const { data } = await api.get("/quiz/shifts");
  const minutes = Number(data?.test_duration);
  return {
    shifts: data?.shifts ?? [],
    durationSeconds: Number.isFinite(minutes) && minutes > 0 ? minutes * 60 : null,
  };
}

/** The questions assigned to this candidate's shift. */
export async function fetchQuestions() {
  const { data } = await api.get("/quiz/get");
  return (data?.questions ?? []).map(normalizeQuestion);
}

/**
 * True once this candidate's attempt has been recorded. The endpoint reports
 * "not submitted" with a 404, and older backends answer 401 when the candidate
 * simply has no tracking row yet — neither is a real failure here, and a truly
 * expired session still surfaces as a 401 from the questions call right after.
 */
export async function checkAlreadySubmitted() {
  try {
    const { data } = await api.post("/quiz/submitted", {});
    return Boolean(data?.submitted);
  } catch (err) {
    const status = err?.response?.status;
    if (status === 404 || status === 401) return false;
    throw err;
  }
}

/**
 * Raises a "waiting for a shift" flag the admin portal can see. Testing aid for
 * before the registration app is live; resolves to {status} which is "pending",
 * "assigned" (a paper already exists) or "none".
 */
export async function requestSlot() {
  const { data } = await api.post("/quiz/request-slot", {});
  return data?.status ?? "pending";
}

/** Where this candidate stands: "assigned", "pending" or "none". */
export async function fetchSlotStatus() {
  const { data } = await api.get("/quiz/slot-status");
  return data?.status ?? "none";
}

/**
 * Maps the answers map back onto the id arrays the backend issued and posts the
 * attempt. Field names match the Go struct (`Image`, `Responses`, `FlagsRaised`).
 */
export async function submitQuiz(questions, answers, flagsRaised, snapshotImage) {
  const responses = [];
  for (const question of questions) {
    const chosenOptionId = answers[question.id];
    if (chosenOptionId === undefined) continue;
    const option = question.options.find((opt) => opt.id === chosenOptionId);
    if (!option || question.rawId == null || option.rawId == null) continue;
    responses.push({ QuestionID: question.rawId, Answer: option.rawId });
  }

  const { data } = await api.post("/quiz/submit", {
    Image: snapshotImage || "",
    Responses: responses,
    FlagsRaised: flagsRaised ?? 0,
  });
  return data;
}

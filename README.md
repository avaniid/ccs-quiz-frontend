# CCS Quiz Portal

A proctored, browser-based quiz portal built with React and Vite. Candidates sign in, grant camera/mic access, and take a timed quiz while a set of anti-cheat guards run in the background and flag suspicious activity.

## Features

- **Timed quiz** with a question palette (answered / marked for review / not answered / not visited), auto-submit on timeout.
- **Proctoring**, all running client-side:
  - Tab switch / window focus loss detection
  - Fullscreen enforcement
  - Face detection — flags when no face or more than one face is in frame
  - Object detection — flags phones, laptops, tablets, and other devices seen in the camera frame
  - Voice matching — a short voice enrollment at the start of the quiz builds a voiceprint; any other voice heard afterward is flagged
- Every violation feeds one shared warning counter; the test is auto-submitted once too many warnings accumulate.

## Tech Stack

- [React 19](https://react.dev/) + [Vite](https://vitejs.dev/)
- [React Router](https://reactrouter.com/)
- [Tailwind CSS v4](https://tailwindcss.com/)
- [@mediapipe/tasks-vision](https://developers.google.com/mediapipe) — face and object detection
- [Meyda](https://meyda.js.org/) — audio feature extraction for voice matching
- [Axios](https://axios-http.com/)

## Getting Started

Requires Node.js (v18+ recommended).

```bash
npm install
npm run dev       # start the dev server
npm run build     # production build
npm run preview   # preview the production build locally
npm run lint      # run ESLint
```

The app requests camera and microphone permissions on the Instructions page before the quiz starts — allow both when prompted.

## Project Structure

```
src/
  api/          axios client (used for auth verification)
  components/   shared UI: WarningModal, VoiceEnrollmentOverlay
  context/      QuizContext — questions, answers, timer, warning count
  hooks/        proctoring guards (camera stream, face, object, voice,
                tab-switch, fullscreen) + the exam timer
  mock/         leftover static question data (unused now that services/api.js
                talks to the real backend)
  pages/        Home, Instructions, Quiz, Submitted, Disqualified,
                LoginRedirect
  services/     the quiz API client (normalises the backend payloads)
backend/        Go + Gin + MongoDB API (see backend/.env.example)
```

## How Proctoring Works

1. **Instructions** page requests camera/mic access as a precondition to starting the quiz.
2. On the **Quiz** page, `useCameraStream` acquires a persistent camera/mic stream shared by every guard.
3. A 10-second **voice enrollment** builds a voiceprint from the candidate's own speech before the quiz becomes interactive.
4. In the background, `useFaceGuard`, `useObjectGuard`, and `useVoiceGuard` continuously watch the camera/mic feed, alongside the existing `useTabSwitchGuard` and `useFullscreenGuard`.
5. Every guard reports through the same `onViolation` callback in `Quiz.jsx`, which increments a shared warning count, shows `WarningModal`, and auto-submits the test once the threshold is hit.

## Backend Integration

The backend lives in `backend/` (Go + Gin + MongoDB) and listens on `:2117`.

```bash
cd backend
cp .env.example .env    # then fill in Mongo, JWT/session secrets and Google OAuth
go run .
```

Copy `.env.example` to `.env` in the frontend too and point `VITE_API_URL` at that
server. Auth is an http-only `session_token` cookie, so the axios client sends
every request `withCredentials`.

### Environment variables that must line up

| Backend | Must be |
| --- | --- |
| `CLIENT_CALLBACK_URL` | `<backend>/google_callback`, registered verbatim in Google Cloud Console |
| `FRONTEND_REDIRECT_URL` | `<frontend>/login-redirect` — the route that verifies the new session |
| `FRONTEND_URL` | the frontend's **origin** only, no path (comma-separate several); `localhost:3000` and `localhost:5173` are always allowed |
| `TEST_DURATION` | exam length in **minutes** (default 80); the frontend converts it to the countdown |

### Endpoints the frontend uses

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/auth/google` | Top-level redirect that starts the OAuth handshake |
| GET | `/verify` | Confirms the session cookie; returns the signed-in email/name |
| GET | `/quiz/shifts` | Public config — shift timings and `test_duration` |
| GET | `/quiz/get` | The candidate's assigned questions (also flags the attempt as started) |
| POST | `/quiz/submitted` | Whether this candidate has already submitted (404 = not yet) |
| POST | `/quiz/submit` | The attempt: snapshot, answers, warning count |
| GET | `/logout` | Clears the session cookie |

### Question and answer payloads

The Go models carry only `bson` tags, so Gin serialises Go field names verbatim
and the `[7]byte` ids arrive as arrays of seven numbers:

```jsonc
// GET /quiz/get
{ "questions": [ { "QuestionID": [1,2,3,4,5,6,7], "Question": "...",
                   "Options": [ { "Value": "4", "Id": [10,0,0,0,0,0,2] } ],
                   "Image": "", "Shift": 1 } ] }

// POST /quiz/submit
{ "Image": "data:image/jpeg;base64,...", "FlagsRaised": 0,
  "Responses": [ { "QuestionID": [1,2,3,4,5,6,7], "Answer": [10,0,0,0,0,0,2] } ] }
```

`src/services/api.js` normalises this into `{ id, rawId, text, image, options }`,
keeping `rawId` untouched because `/quiz/submit` validates answers against the
exact ids it handed out. It also accepts lowercase/bson spellings, so adding
`json` tags to the Go models later won't break the frontend.

## Notes

- Flow: `/` → `/auth/google` → Google → `/google_callback` → `/login-redirect` →
  `/instructions` → `/quiz` → `/submitted`.
- The backend marks `quiz_submitted` in a background goroutine, so
  `POST /quiz/submitted` can still read `false` for a few milliseconds
  immediately after a successful submit.
- `VITE_API_URL` is read by `src/api/client.js` for the axios base URL and
  defaults to `http://localhost:2117`.

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
  mock/         mock question data (fetchQuestions/submitQuiz are mocked
                for now — see src/services/api.js)
  pages/        Home, Instructions, Quiz, Submitted, Disqualified,
                LoginRedirect
  services/     the mock quiz API
```

## How Proctoring Works

1. **Instructions** page requests camera/mic access as a precondition to starting the quiz.
2. On the **Quiz** page, `useCameraStream` acquires a persistent camera/mic stream shared by every guard.
3. A 10-second **voice enrollment** builds a voiceprint from the candidate's own speech before the quiz becomes interactive.
4. In the background, `useFaceGuard`, `useObjectGuard`, and `useVoiceGuard` continuously watch the camera/mic feed, alongside the existing `useTabSwitchGuard` and `useFullscreenGuard`.
5. Every guard reports through the same `onViolation` callback in `Quiz.jsx`, which increments a shared warning count, shows `WarningModal`, and auto-submits the test once the threshold is hit.

## Notes

- `src/services/api.js` currently mocks the backend (`fetchQuestions` returns static mock data, `submitQuiz` just logs the payload) — wire these up to a real API when the backend is ready.
- `LoginRedirect.jsx` and `Disqualified.jsx` exist but aren't wired into `App.jsx`'s routes yet.
- `VITE_API_URL` is read by `src/api/client.js` for the axios base URL.

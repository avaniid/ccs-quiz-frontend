// One camera/mic stream for the whole attempt.
//
// The instructions page needs to ask for permission before letting anyone in,
// and the quiz page needs a live stream for the proctoring guards. Doing that as
// two separate getUserMedia calls meant the device was opened, closed, then
// reopened a moment later — and Windows regularly fails that second open with
// NotReadableError ("Could not start video source") because the OS has not
// released the camera yet. On a machine where the permission was already granted
// the two calls happen to line up, which is why this only showed up on a fresh
// origin.
//
// So the stream is acquired once and handed to both pages.

const CONSTRAINTS = {
  video: { width: 640, height: 480, facingMode: "user" },
  audio: { channelCount: 1, echoCancellation: false, noiseSuppression: false },
};

let current = null;
let inFlight = null;

function isLive(stream) {
  return Boolean(stream) && stream.getTracks().some((t) => t.readyState === "live");
}

/**
 * Resolves with the shared stream, opening the device only if there isn't
 * already a live one. Concurrent callers share a single request.
 */
export function acquireCameraStream() {
  if (isLive(current)) return Promise.resolve(current);
  if (inFlight) return inFlight;

  inFlight = navigator.mediaDevices
    .getUserMedia(CONSTRAINTS)
    .then((stream) => {
      current = stream;
      inFlight = null;
      return stream;
    })
    .catch((err) => {
      inFlight = null;
      throw err;
    });

  return inFlight;
}

/** Stops the shared stream. Called when the attempt is over. */
export function releaseCameraStream() {
  if (current) {
    current.getTracks().forEach((track) => track.stop());
    current = null;
  }
}

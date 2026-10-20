// Watches the shared camera/mic stream for the candidate cutting it off.
//
// Listening for the track's "ended" event alone missed almost every real way of
// doing that:
//   - Blocking the camera from the address bar flips the permission to
//     "denied"; not every browser ends the track for it.
//   - The Windows privacy toggle or a laptop's camera kill switch MUTES the
//     track instead of ending it, and the <video> element freezes on its last
//     frame. Face detection then keeps finding the candidate's face in that
//     frozen frame, so nothing was flagged at all.
// So every second this checks that each track is live and unmuted, that the
// permission has not been revoked, and that the camera is still producing new
// frames. A covered lens still produces (dark) frames; the face guard reports
// that one as "face-missing".
//
// Kept free of React and the DOM globals it can be handed, so it can be tested
// with fake tracks and a fake clock.

export const CAMERA_BLOCKED = "camera-blocked";
export const MIC_BLOCKED = "mic-blocked";

export function createDeviceWatchdog({
  stream,
  getVideo,
  onViolation,
  now = () => performance.now(),
  isHidden = () => typeof document !== "undefined" && document.hidden,
  permissions = typeof navigator !== "undefined" ? navigator.permissions : undefined,
  setIntervalFn = setInterval,
  clearIntervalFn = clearInterval,
  intervalMs = 1000,
  // No new camera frame for this long while the page is visible = feed is dead.
  stallMs = 3000,
  // While a device stays blocked, raise it again this often. Each one is a
  // strike, so leaving the camera off runs the attempt into auto-submit.
  repeatMs = 5000,
}) {
  const perm = { camera: "unknown", microphone: "unknown" };
  const lastFlagAt = { camera: -Infinity, mic: -Infinity };
  const permStatuses = [];
  const trackListeners = [];

  let boundVideo = null;
  let frameHandle = null;
  let lastFrameAt = now();
  let lastFrameMark = null;
  let disposed = false;

  const videoTrack = () => stream.getVideoTracks()[0];
  const audioTrack = () => stream.getAudioTracks()[0];
  const trackDown = (t) => !t || t.readyState !== "live" || t.muted;

  // Follow whichever <video> currently shows the stream. Quiz.jsx mounts it
  // only once the paper is on screen, so it can appear (or change) later.
  function followFrames() {
    const video = getVideo();
    if (video !== boundVideo) {
      if (boundVideo && frameHandle != null && boundVideo.cancelVideoFrameCallback) {
        boundVideo.cancelVideoFrameCallback(frameHandle);
      }
      boundVideo = video;
      frameHandle = null;
      lastFrameMark = null;
      lastFrameAt = now();
      if (video && typeof video.requestVideoFrameCallback === "function") {
        const onFrame = () => {
          if (disposed || boundVideo !== video) return;
          lastFrameAt = now();
          frameHandle = video.requestVideoFrameCallback(onFrame);
        };
        frameHandle = video.requestVideoFrameCallback(onFrame);
      }
    }

    // Fallback for browsers without requestVideoFrameCallback.
    if (boundVideo && typeof boundVideo.requestVideoFrameCallback !== "function") {
      const mark =
        boundVideo.getVideoPlaybackQuality?.().totalVideoFrames ?? boundVideo.currentTime;
      if (mark !== lastFrameMark) {
        lastFrameMark = mark;
        lastFrameAt = now();
      }
    }

    // A background tab stops rendering video frames on purpose. The tab-switch
    // guard already flags leaving the tab, so do not also call it a dead camera,
    // and do not count the hidden time once the candidate comes back.
    if (isHidden()) lastFrameAt = now();
  }

  function cameraBlocked() {
    if (trackDown(videoTrack())) return true;
    if (perm.camera === "denied") return true;
    return Boolean(boundVideo) && now() - lastFrameAt > stallMs;
  }

  function micBlocked() {
    return trackDown(audioTrack()) || perm.microphone === "denied";
  }

  function raise(key, type, blocked) {
    if (!blocked) {
      // Recovered: a later block is reported straight away, not after a cooldown.
      lastFlagAt[key] = -Infinity;
      return;
    }
    const t = now();
    if (t - lastFlagAt[key] >= repeatMs) {
      lastFlagAt[key] = t;
      onViolation(type);
    }
  }

  function check() {
    if (disposed) return;
    followFrames();
    raise("camera", CAMERA_BLOCKED, cameraBlocked());
    raise("mic", MIC_BLOCKED, micBlocked());
  }

  // React to the events immediately instead of waiting for the next tick.
  for (const track of stream.getTracks()) {
    for (const evt of ["ended", "mute"]) {
      track.addEventListener(evt, check);
      trackListeners.push([track, evt]);
    }
  }

  for (const name of ["camera", "microphone"]) {
    permissions
      ?.query({ name })
      .then((status) => {
        if (disposed) return;
        perm[name] = status.state;
        status.onchange = () => {
          perm[name] = status.state;
          check();
        };
        permStatuses.push(status);
      })
      // Not every browser can query camera/microphone permissions; the track
      // and frame checks still cover those.
      .catch(() => {});
  }

  const timer = setIntervalFn(check, intervalMs);

  return {
    check,
    dispose() {
      disposed = true;
      clearIntervalFn(timer);
      for (const [track, evt] of trackListeners) track.removeEventListener(evt, check);
      for (const status of permStatuses) status.onchange = null;
      if (boundVideo && frameHandle != null && boundVideo.cancelVideoFrameCallback) {
        boundVideo.cancelVideoFrameCallback(frameHandle);
      }
    },
  };
}

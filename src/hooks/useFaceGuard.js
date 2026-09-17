import { useEffect, useRef } from "react";
import { getVisionFileset } from "./mediapipeVision";

const MODEL_URL =
  "https://storage.googleapis.com/mediapipe-models/face_landmarker/face_landmarker/float16/1/face_landmarker.task";

const CHECK_INTERVAL_MS = 700;
const MISS_THRESHOLD = 2;
const VIOLATION_COOLDOWN_MS = 4000;

export default function useFaceGuard(videoRef, stream, onViolation) {
  const onViolationRef = useRef(onViolation);
  onViolationRef.current = onViolation;

  useEffect(() => {
    if (!stream) return;

    let cancelled = false;
    let rafId = null;
    let landmarker = null;
    let lastCheckTs = 0;
    let missCount = 0;
    let lastMissingViolationAt = 0;
    let lastMultiFaceViolationAt = 0;

    async function setup() {
      try {
        const { FaceLandmarker } = await import("@mediapipe/tasks-vision");
        const vision = await getVisionFileset();
        if (cancelled) return;
        landmarker = await FaceLandmarker.createFromOptions(vision, {
          baseOptions: { modelAssetPath: MODEL_URL, delegate: "GPU" },
          runningMode: "VIDEO",
          numFaces: 3,
          outputFaceBlendshapes: false,
          outputFacialTransformationMatrixes: false,
        });
      } catch (err) {
        console.warn("Face detection model failed to load:", err);
        return;
      }
      if (cancelled) return;
      loop();
    }

    function loop(ts = performance.now()) {
      if (cancelled) return;
      rafId = requestAnimationFrame(loop);

      const video = videoRef.current;
      if (!landmarker || !video || video.videoWidth === 0) return;
      if (ts - lastCheckTs < CHECK_INTERVAL_MS) return;
      lastCheckTs = ts;

      let result;
      try {
        result = landmarker.detectForVideo(video, ts);
      } catch (err) {
        console.warn("Face detection frame failed:", err);
        return;
      }

      const faceCount = result.faceLandmarks.length;

      if (faceCount === 0) {
        missCount += 1;
        if (missCount >= MISS_THRESHOLD && ts - lastMissingViolationAt > VIOLATION_COOLDOWN_MS) {
          lastMissingViolationAt = ts;
          onViolationRef.current?.("face-missing");
        }
      } else {
        missCount = 0;
      }

      if (faceCount > 1 && ts - lastMultiFaceViolationAt > VIOLATION_COOLDOWN_MS) {
        lastMultiFaceViolationAt = ts;
        onViolationRef.current?.("multiple-faces");
      }
    }

    setup();

    return () => {
      cancelled = true;
      if (rafId) cancelAnimationFrame(rafId);
      landmarker?.close();
    };
  }, [stream, videoRef]);
}

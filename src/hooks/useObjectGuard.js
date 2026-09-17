import { useEffect, useRef } from "react";
import { getVisionFileset } from "./mediapipeVision";

const MODEL_URL =
  "https://storage.googleapis.com/mediapipe-models/object_detector/efficientdet_lite0/float32/1/efficientdet_lite0.tflite";

const CHECK_INTERVAL_MS = 2200;
const VIOLATION_COOLDOWN_MS = 5000;
const FLAGGED_LABELS = ["cell phone", "laptop", "tablet", "remote", "book", "mouse"];

export default function useObjectGuard(videoRef, stream, onViolation) {
  const onViolationRef = useRef(onViolation);
  onViolationRef.current = onViolation;

  useEffect(() => {
    if (!stream) return;

    let cancelled = false;
    let rafId = null;
    let detector = null;
    let lastCheckTs = 0;
    let lastViolationAt = 0;

    async function setup() {
      try {
        const { ObjectDetector } = await import("@mediapipe/tasks-vision");
        const vision = await getVisionFileset();
        if (cancelled) return;
        detector = await ObjectDetector.createFromOptions(vision, {
          // CPU here, not GPU: FaceLandmarker (useFaceGuard) already holds
          // the GPU delegate context, and MediaPipe's web GPU delegate
          // doesn't reliably support two concurrent GPU-backed tasks in one
          // tab — the second one to init can silently fail. CPU avoids the
          // contention entirely; EfficientDet-Lite0 is small enough that
          // CPU inference is still fast enough for a ~2s polling interval.
          baseOptions: { modelAssetPath: MODEL_URL, delegate: "CPU" },
          runningMode: "VIDEO",
          maxResults: 5,
          scoreThreshold: 0.42,
        });
      } catch (err) {
        console.warn("Object detection model failed to load:", err);
        return;
      }
      if (cancelled) return;
      console.log("[object-guard] detector running");
      loop();
    }

    function loop(ts = performance.now()) {
      if (cancelled) return;
      rafId = requestAnimationFrame(loop);

      const video = videoRef.current;
      if (!detector || !video || video.videoWidth === 0) return;
      if (ts - lastCheckTs < CHECK_INTERVAL_MS) return;
      lastCheckTs = ts;

      let result;
      try {
        result = detector.detectForVideo(video, ts);
      } catch (err) {
        console.warn("Object detection frame failed:", err);
        return;
      }

      if (result.detections.length > 0) {
        const seen = result.detections
          .map((d) => `${d.categories[0]?.categoryName} (${(d.categories[0]?.score * 100).toFixed(0)}%)`)
          .join(", ");
        console.log(`[object-guard] sees: ${seen}`);
      }

      if (ts - lastViolationAt <= VIOLATION_COOLDOWN_MS) return;

      const hit = result.detections.find((d) => {
        const label = (d.categories[0]?.categoryName || "").toLowerCase();
        return FLAGGED_LABELS.some((l) => label.includes(l));
      });

      if (hit) {
        lastViolationAt = ts;
        onViolationRef.current?.("device-detected");
      }
    }

    setup();

    return () => {
      cancelled = true;
      if (rafId) cancelAnimationFrame(rafId);
      detector?.close();
    };
  }, [stream, videoRef]);
}

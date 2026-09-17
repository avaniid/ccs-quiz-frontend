import { useCallback, useEffect, useRef, useState } from "react";

const DEFAULT_VAD_RMS_THRESHOLD = 0.012;
const MIN_VAD_RMS_THRESHOLD = 0.006;
const MAX_VAD_RMS_THRESHOLD = 0.05;
const VOICE_SIM_THRESHOLD = 0.87;
export const ENROLL_SECONDS = 10;
const MFCC_BINS = 13;
// Live-matching (not "wait for the speaker to pause") tuning: compare a
// short rolling window of the most recent frames every COMPARE_INTERVAL_MS
// while someone is talking, instead of waiting for a silence gap before
// comparing — that wait was the reported "detection is late" delay.
const ROLLING_WINDOW_FRAMES = 10;
const MIN_FRAMES_TO_COMPARE = 5;
const COMPARE_INTERVAL_MS = 400;
const MISMATCH_COOLDOWN_MS = 3000;

function averageMfcc(frames) {
  const avg = new Array(MFCC_BINS).fill(0);
  for (const frame of frames) {
    frame.forEach((v, i) => {
      avg[i] += v;
    });
  }
  return avg.map((v) => v / frames.length);
}

function cosineSim(a, b) {
  let dot = 0;
  let magA = 0;
  let magB = 0;
  for (let i = 0; i < a.length; i++) {
    dot += a[i] * b[i];
    magA += a[i] * a[i];
    magB += b[i] * b[i];
  }
  if (magA === 0 || magB === 0) return 0;
  return dot / (Math.sqrt(magA) * Math.sqrt(magB));
}

export default function useVoiceGuard(stream, onViolation) {
  const onViolationRef = useRef(onViolation);
  onViolationRef.current = onViolation;

  const [phase, setPhase] = useState("idle"); // idle | enrolling | ready
  const [secondsLeft, setSecondsLeft] = useState(ENROLL_SECONDS);

  const phaseRef = useRef("idle");
  const audioCtxRef = useRef(null);
  const analyzerRef = useRef(null);
  const enrollFramesRef = useRef([]); // [{ rms, mfcc }]
  const vadThresholdRef = useRef(DEFAULT_VAD_RMS_THRESHOLD);
  const voiceprintRef = useRef(null);
  const rollingFramesRef = useRef([]);
  const lastCompareAtRef = useRef(0);
  const lastMismatchAtRef = useRef(0);
  const enrollTimerRef = useRef(null);

  // Set up the analyzer once the mic track is available; stays alive for
  // the whole quiz session and just changes behavior based on phaseRef.
  useEffect(() => {
    if (!stream) return;
    const audioTrack = stream.getAudioTracks()[0];
    if (!audioTrack) return;

    let meydaMod;
    let cancelled = false;

    async function setup() {
      try {
        meydaMod = await import("meyda");
        if (cancelled) return;
        const Meyda = meydaMod.default || meydaMod;

        const audioCtx = new (window.AudioContext || window.webkitAudioContext)();
        if (audioCtx.state === "suspended") {
          await audioCtx.resume().catch(() => {});
        }
        const source = audioCtx.createMediaStreamSource(stream);
        audioCtxRef.current = audioCtx;

        analyzerRef.current = Meyda.createMeydaAnalyzer({
          audioContext: audioCtx,
          source,
          bufferSize: 512,
          featureExtractors: ["mfcc", "rms"],
          numberOfMFCCCoefficients: MFCC_BINS,
          callback: onFrame,
        });
        analyzerRef.current.start();
        console.log(`[voice-guard] Meyda analyzer running (audioContext state: ${audioCtx.state})`);
      } catch (err) {
        console.error("[voice-guard] failed to start audio analysis:", err);
      }
    }

    function onFrame(features) {
      if (!features) return;
      const { rms, mfcc } = features;

      if (phaseRef.current === "enrolling") {
        // Collect every frame (not just ones above a guessed threshold) so
        // the voiceprint always forms — mic sensitivity varies too much
        // across devices to gate this on a fixed RMS constant. rms is kept
        // alongside each frame so finishEnrollment can both calibrate the
        // active-phase VAD threshold AND pick out the actually-voiced frames
        // for the voiceprint itself.
        if (mfcc) enrollFramesRef.current.push({ rms, mfcc: [...mfcc] });
        return;
      }

      if (phaseRef.current !== "ready") return;

      if (rms <= vadThresholdRef.current) {
        // Silence: drop the window so the next burst of speech starts clean
        // instead of blending with whoever was talking before.
        rollingFramesRef.current = [];
        return;
      }

      if (!mfcc) return;
      rollingFramesRef.current.push([...mfcc]);
      if (rollingFramesRef.current.length > ROLLING_WINDOW_FRAMES) {
        rollingFramesRef.current.shift();
      }

      if (
        !voiceprintRef.current ||
        rollingFramesRef.current.length < MIN_FRAMES_TO_COMPARE
      ) {
        return;
      }

      const now = performance.now();
      if (now - lastCompareAtRef.current < COMPARE_INTERVAL_MS) return;
      lastCompareAtRef.current = now;

      const windowVoice = averageMfcc(rollingFramesRef.current);
      const sim = cosineSim(voiceprintRef.current, windowVoice);
      console.log(`[voice-guard] live similarity: ${sim.toFixed(3)}`);

      if (
        sim < VOICE_SIM_THRESHOLD &&
        now - lastMismatchAtRef.current > MISMATCH_COOLDOWN_MS
      ) {
        lastMismatchAtRef.current = now;
        onViolationRef.current?.("voice-mismatch");
      }
    }

    setup();

    return () => {
      cancelled = true;
      analyzerRef.current?.stop();
      audioCtxRef.current?.close();
      audioCtxRef.current = null;
      analyzerRef.current = null;
    };
  }, [stream]);

  const startEnrollment = useCallback(() => {
    if (phaseRef.current !== "idle") return;
    phaseRef.current = "enrolling";
    setPhase("enrolling");
    enrollFramesRef.current = [];

    const startedAt = Date.now();
    const durationMs = ENROLL_SECONDS * 1000;

    enrollTimerRef.current = setInterval(() => {
      const remaining = Math.max(durationMs - (Date.now() - startedAt), 0);
      setSecondsLeft(Math.ceil(remaining / 1000));
      if (remaining <= 0) {
        clearInterval(enrollTimerRef.current);
        finishEnrollment();
      }
    }, 100);

    function finishEnrollment() {
      const captured = enrollFramesRef.current;

      if (captured.length === 0) {
        console.warn(
          "[voice-guard] enrollment captured NO audio frames at all — mic analyzer likely isn't running"
        );
        phaseRef.current = "ready";
        setPhase("ready");
        return;
      }

      const rmsValues = captured.map((f) => f.rms);
      const avgRms = rmsValues.reduce((a, b) => a + b, 0) / rmsValues.length;
      const peakRms = Math.max(...rmsValues);
      // Speech frames sit well above the average (which is diluted by
      // pauses/silence), so calibrate relative to the peak instead of a
      // fixed constant that assumes a specific mic gain. Capped both ends so
      // one quiet mic or one loud noise burst during enrollment can't make
      // the gate impossibly strict (missing real speech) or impossibly loose
      // (letting a second voice through unnoticed).
      vadThresholdRef.current = Math.max(
        MIN_VAD_RMS_THRESHOLD,
        Math.min(avgRms * 1.8, peakRms * 0.35, MAX_VAD_RMS_THRESHOLD)
      );
      console.log(
        `[voice-guard] mic calibration — avgRms=${avgRms.toFixed(4)} peakRms=${peakRms.toFixed(
          4
        )} → vadThreshold=${vadThresholdRef.current.toFixed(4)}`
      );

      // Build the voiceprint from only the loudest (most clearly-voiced)
      // half of the captured frames, not just everything above the coarse
      // VAD gate — a purer reference makes the enrolled voice easier to
      // tell apart from a different speaker later.
      const sorted = [...captured].sort((a, b) => b.rms - a.rms);
      const source = sorted.slice(0, Math.max(1, Math.ceil(sorted.length / 2)));
      voiceprintRef.current = averageMfcc(source.map((f) => f.mfcc));
      console.log(
        `[voice-guard] voiceprint built from ${source.length}/${captured.length} loudest frames`
      );

      phaseRef.current = "ready";
      setPhase("ready");
    }
  }, []);

  useEffect(() => {
    return () => {
      if (enrollTimerRef.current) clearInterval(enrollTimerRef.current);
    };
  }, []);

  return { phase, secondsLeft, startEnrollment };
}

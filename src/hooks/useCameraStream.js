import { useEffect, useRef, useState } from "react";

export default function useCameraStream(onViolation) {
  const onViolationRef = useRef(onViolation);
  onViolationRef.current = onViolation;

  const videoRef = useRef(null);
  const streamRef = useRef(null);
  const [stream, setStream] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    let cancelled = false;

    const handleTrackEnded = () => {
      onViolationRef.current?.("camera-lost");
    };

    async function start() {
      try {
        const mediaStream = await navigator.mediaDevices.getUserMedia({
          video: { width: 640, height: 480, facingMode: "user" },
          audio: { channelCount: 1, echoCancellation: false, noiseSuppression: false },
        });
        if (cancelled) {
          mediaStream.getTracks().forEach((track) => track.stop());
          return;
        }
        streamRef.current = mediaStream;
        mediaStream.getTracks().forEach((track) => {
          track.addEventListener("ended", handleTrackEnded);
        });
        setStream(mediaStream);
      } catch (err) {
        if (!cancelled) {
          setError(err);
        }
      }
    }

    start();

    return () => {
      cancelled = true;
      const mediaStream = streamRef.current;
      if (mediaStream) {
        mediaStream.getTracks().forEach((track) => {
          track.removeEventListener("ended", handleTrackEnded);
          track.stop();
        });
        streamRef.current = null;
      }
    };
  }, []);

  // The <video> element only mounts once `stream` is set (Quiz.jsx renders it
  // conditionally), so videoRef.current is still null at the point start()
  // resolves above. Bind srcObject here instead, once the element exists.
  useEffect(() => {
    if (videoRef.current && stream) {
      videoRef.current.srcObject = stream;
    }
  }, [stream]);

  return { stream, videoRef, error };
}

import { useEffect, useRef, useState } from "react";
import { acquireCameraStream } from "./cameraStore";

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
        // Reuses the stream the instructions page already opened, instead of
        // closing the device and racing to reopen it.
        const mediaStream = await acquireCameraStream();
        if (cancelled) return;
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
        });
        streamRef.current = null;
      }
      // Deliberately NOT releasing the device here. React remounts this effect
      // (StrictMode in development, and any re-render of the route), and
      // stopping the camera only to reopen it a moment later is exactly the
      // race that broke proctoring. The stream is released once the attempt
      // ends — see submitQuiz in Quiz.jsx — and the browser reclaims it when
      // the tab closes.
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
